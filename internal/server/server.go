package server

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	goradiko "github.com/yyoshiki41/go-radiko"
	"log/slog"
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
	channelMap := make(map[string]Channel)
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

		channel, ok := channelMap[prog.Title]
		if !ok {
			channel = Channel{
				Title:     prog.Title,
				Generator: "abekoh/radiko-podcast",
				Owner: ITunesOwner{
					Name: prog.Pfm,
				},
				Language: "ja",
				Item:     []Item{},
			}
			channelMap[prog.Title] = channel
		}
		startTime, err := time.ParseInLocation("20060102150405", prog.Ft, JST)
		if err != nil {
			return fmt.Errorf("failed to parse start time: %w", err)
		}
		endTime, err := time.ParseInLocation("20060102150405", prog.To, JST)
		if err != nil {
			return fmt.Errorf("failed to parse end time: %w", err)
		}
		channel.Item = append(channel.Item, Item{
			Title:       prog.Title,
			Description: prog.Info,
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

	channels := make([]Channel, 0, len(channelMap))
	for _, channel := range channelMap {
		channels = append(channels, channel)
	}
	rs := &RSS{
		Channel: channels,
	}
	logger.Debug("complete generating RSS", "rss", rs)
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
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/xml")

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(rss); err != nil {
		http.Error(w, "Failed to encode XML", http.StatusInternalServerError)
	}
}
