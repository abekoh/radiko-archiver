package radiko

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	goradiko "github.com/yyoshiki41/go-radiko"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

func RunFetchers(ctx context.Context, toFetcher <-chan Schedule, outDirPath string) {
	logger := slog.Default().With("job", "fetchers")
	logger.Debug("start fetchers")

	radikoClient, err := goradiko.New("")
	if err != nil {
		panic(fmt.Errorf("failed to create radiko client: %w", err))
	}
	_, err = radikoClient.AuthorizeToken(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to authorize token: %w", err))
	}

	go func() {
		for {
			select {
			case sche := <-toFetcher:
				c, cancel := context.WithTimeout(ctx, fetchTimeout)
				defer cancel()
				go func(ctx context.Context, s Schedule, log *slog.Logger) {
					workingDirPath := os.TempDir()

					if err := fetch(ctx, s, radikoClient, outDirPath, workingDirPath); err != nil {
						log.Error("failed to fetch", "error", err)
						return
					}

					if err := convert(ctx, s, outDirPath, workingDirPath); err != nil {
						log.Error("failed to convert", "error", err)
						return
					}

				}(c, sche, logger.With("job", "fetcher-"+time.Now().Format("20060102150405")))
			case <-ctx.Done():
				logger.Debug("stop fetchers")
				return
			}
		}
	}()
}

const (
	maxAttempts    = 3
	maxConcurrents = 64
)

func fetch(ctx context.Context, s Schedule, radikoClient *goradiko.Client, outDirPath string, workingDirPath string) error {
	logger := slog.Default().With("job", fmt.Sprintf("fetcher-%s-%s", s.StationID, s.StartTime.Format("20060102150405")))

	logger.Info("start fetching", "schedule", s)
	pg, err := radikoClient.GetProgramByStartTime(ctx, string(s.StationID), s.StartTime)
	if err != nil {
		return fmt.Errorf("failed to fetch program: %w", err)
	}
	logger.Debug("get program", "program", pg)

	xmlFile, err := os.Create(fmt.Sprintf("%s/%s_%s_%s.xml", outDirPath, pg.Ft, s.StationID, pg.Title))
	if err != nil {
		logger.Error("failed to create file", "error", err)
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer xmlFile.Close()
	xmlEncoder := xml.NewEncoder(xmlFile)
	xmlEncoder.Indent("", "  ")
	if err := xmlEncoder.Encode(pg); err != nil {
		return fmt.Errorf("failed to encode xml: %w", err)
	}

	m3u8URI, err := radikoClient.TimeshiftPlaylistM3U8(ctx, string(s.StationID), s.StartTime)
	if err != nil {
		return fmt.Errorf("failed to get m3u8URI: %w", err)
	}
	logger.Debug("got m3u8URI", "m3u8URI", m3u8URI)

	chunkList, err := goradiko.GetChunklistFromM3U8(m3u8URI)
	if err != nil {
		return fmt.Errorf("failed to get chunkList: %w", err)
	}
	logger.Debug("got chunkList", "chunkList[:5]", chunkList[:5], "len()", len(chunkList))

	logger.Debug("workingDirPath", "workingDirPath", workingDirPath)
	if err := bulkDownload(
		ctx,
		chunkList,
		workingDirPath,
		string(s.StationID)+s.StartTime.Format("20060102150405")+"_",
	); err != nil {
		return fmt.Errorf("failed to download chunks: %w", err)
	}
	logger.Debug("complete downloading chunks")

	logger.Info("finish fetching")

	return nil
}

func bulkDownload(ctx context.Context, urls []string, outDirPath, fileNamePrefix string) error {
	sem := semaphore.NewWeighted(maxConcurrents)
	g, ctx := errgroup.WithContext(ctx)
	for _, url := range urls {
		url := url
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to acquire semaphore: %w", err)
			}
			defer sem.Release(1)

			attempts := 0
			for {
				attempts++
				_, urlFilename := filepath.Split(url)
				if err := download(ctx, url, outDirPath, fileNamePrefix+urlFilename); err != nil {
					if attempts >= maxAttempts {
						return fmt.Errorf("failed to download: %w", err)
					}
				} else {
					break
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to download one job: %w", err)
	}
	return nil
}

func download(ctx context.Context, url, outDirPath, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	file, err := os.Create(filepath.Join(outDirPath, filename))
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return err
}

func convert(ctx context.Context, s Schedule, outDirPath string, workingDirPath string) error {
	logger := slog.Default().With("job", fmt.Sprintf("converter-%s-%s", s.StationID, s.StartTime.Format("20060102150405")))
	logger.Info("start converting", "schedule", s)
	tempResourcesFile, err := os.CreateTemp(workingDirPath, "resources_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create resources file: %w", err)
	}
	defer func() {
		_ = os.Remove(tempResourcesFile.Name())
	}()
	var aacFilePaths []string
	if err := filepath.WalkDir(workingDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".aac" {
			aacFilePaths = append(aacFilePaths, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk tempDir: %w", err)
	}
	slices.Sort(aacFilePaths)

	for _, aacFilePath := range aacFilePaths {
		tempResourcesFile.WriteString("file '" + aacFilePath + "'\n")
	}
	tempResourcesFile.Close()

	concatFilePath := filepath.Join(outDirPath, fmt.Sprintf("%s_%s.aac", s.StationID, s.StartTime.Format("20060102150405")))
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-y",
		"-i", tempResourcesFile.Name(),
		"-c", "copy",
		concatFilePath,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to concat aac files: %w", err)
	}
	logger.Debug("complete concat aac files")
	return nil
}
