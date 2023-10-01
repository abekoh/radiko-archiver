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
					log.Info("start fetching", "schedule", s)
					pg, err := radikoClient.GetProgramByStartTime(ctx, string(s.StationID), s.StartTime)
					if err != nil {
						log.Error("failed to fetch program", "error", err)
						return
					}
					log.Debug("get program", "program", pg)

					xmlFile, err := os.Create(fmt.Sprintf("%s/%s_%s_%s.xml", outDirPath, pg.Ft, s.StationID, pg.Title))
					if err != nil {
						log.Error("failed to create file", "error", err)
						return
					}
					defer xmlFile.Close()
					xmlEncoder := xml.NewEncoder(xmlFile)
					xmlEncoder.Indent("", "  ")
					if err := xmlEncoder.Encode(pg); err != nil {
						log.Error("failed to encode xml", "error", err)
						return
					}

					m3u8URI, err := radikoClient.TimeshiftPlaylistM3U8(ctx, string(s.StationID), s.StartTime)
					if err != nil {
						log.Error("failed to get m3u8URI", "error", err)
						return
					}
					log.Debug("got m3u8URI", "m3u8URI", m3u8URI)

					chunkList, err := goradiko.GetChunklistFromM3U8(m3u8URI)
					if err != nil {
						log.Error("failed to get chunkList", "error", err)
						return
					}
					log.Debug("got chunkList", "chunkList[:5]", chunkList[:5], "len()", len(chunkList))

					tempDirPath := os.TempDir()
					log.Debug("tempDirPath", "tempDirPath", tempDirPath)
					if err := bulkDownload(
						ctx,
						chunkList,
						tempDirPath,
						string(s.StationID)+s.StartTime.Format("20060102150405")+"_",
					); err != nil {
						log.Error("failed to download chunks", "error", err)
						return
					}
					log.Debug("complete downloading chunks")

					log.Info("finish fetching")

					tempResourcesFile, err := os.CreateTemp(tempDirPath, "resources_*.txt")
					if err != nil {
						log.Error("failed to create resources file", "error", err)
						return
					}
					defer func() {
						_ = os.Remove(tempResourcesFile.Name())
					}()
					var aacFilePaths []string
					if err := filepath.WalkDir(tempDirPath, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							log.Warn("failed to walk path", "path", path, "error", err)
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
						log.Error("failed to walk tempDir", "error", err)
						return
					}
					slices.Sort(aacFilePaths)

					for _, aacFilePath := range aacFilePaths {
						tempResourcesFile.WriteString("file '" + aacFilePath + "'\n")
					}
					tempResourcesFile.Close()

					concatFilePath := filepath.Join(tempDirPath, "concat.aac")
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
						log.Error("failed to concat aac files", "error", err)
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
