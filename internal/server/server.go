package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log/slog"

	"github.com/fsnotify/fsnotify"
	goradiko "github.com/yyoshiki41/go-radiko"
)

var (
	rss   *RSS
	rssMu sync.RWMutex
)

func RunServer(ctx context.Context, outDirPath string) {
	go func() {
		updateRSS(ctx, outDirPath)
	}()
	go func() {
		http.HandleFunc("/", getRSS)
		http.ListenAndServe(":8080", nil)
	}()
}

func updateRSS(ctx context.Context, outDirPath string) {
	logger := slog.Default().With("job", "updateRSS")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(fmt.Errorf("failed to create watcher: %w", err))
	}
	defer watcher.Close()
	if err := watcher.Add(outDirPath); err != nil {
		panic(fmt.Errorf("failed to add watcher: %w", err))
	}
	rs, err := generateRSS(outDirPath)
	if err != nil {
		panic(fmt.Errorf("failed to generate RSS: %w", err))
	}
	rssMu.Lock()
	rss = rs
	rssMu.Unlock()

	for {
		select {
		case <-watcher.Events:
			rs, err := generateRSS(outDirPath)
			if err != nil {
				logger.Error("failed to generate RSS", "error", err)
				continue
			}
			rssMu.Lock()
			rss = rs
			rssMu.Unlock()
		case err := <-watcher.Errors:
			panic(fmt.Errorf("failed to watch: %w", err))
		case <-ctx.Done():
			return
		}
	}
}

func generateRSS(outDirPath string) (*RSS, error) {
	logger := slog.Default().With("job", "generateRSS")
	var items []Item
	if err := filepath.WalkDir(outDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".xml" {
			return nil
		}
		aacFilePath := path[:len(path)-4] + ".aac"
		if _, err := os.Stat(aacFilePath); os.IsNotExist(err) {
			return fmt.Errorf("failed to find aac file: %w", err)
		}

		xmlFile, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		var prog goradiko.Prog
		if err := xml.Unmarshal(xmlFile, &prog); err != nil {
			return fmt.Errorf("failed to unmarshal xml: %w", err)
		}

		startTime, err := time.ParseInLocation("20060102150405", prog.Ft, JST)
		if err != nil {
			return fmt.Errorf("failed to parse start time: %w", err)
		}
		endTime, err := time.ParseInLocation("20060102150405", prog.To, JST)
		if err != nil {
			return fmt.Errorf("failed to parse end time: %w", err)
		}
		items = append(items, Item{
			Title:       prog.Title,
			Description: "<![CDATA[ " + prog.Info + "]]>",
			PubDate:     startTime.Format(time.RFC1123Z),
			Link:        prog.URL,
			GUID:        GUID{},
			Author:      prog.Pfm,
			Subtitle:    prog.SubTitle,
			Duration:    formatDuration(endTime.Sub(startTime)),
		})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk: %w", err)
	}

	rs := &RSS{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Itunes:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Media:   "http://search.yahoo.com/mrss/",
		DC:      "http://purl.org/dc/elements/1.1/",
		Channel: Channel{
			Title:       "abekoh's Podcast feed",
			Description: "Podcast feed for abekoh",
			Generator:   "abekoh/radiko-podcast",
			Owner: ITunesOwner{
				Name: "abekoh",
			},
			Language: "ja",
			Items:    items,
		},
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.Encode(rs); err != nil {
		return nil, fmt.Errorf("failed to encode xml: %w", err)
	}
	logger.Debug("generated RSS", "rss", buf.String())
	return rs, nil
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func getRSS(w http.ResponseWriter, r *http.Request) {
	rssMu.RLock()
	defer rssMu.RUnlock()
	if rss == nil {
		http.Error(w, "RSS is not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Add("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(rss); err != nil {
		http.Error(w, "Failed to encode XML", http.StatusInternalServerError)
	}
}
