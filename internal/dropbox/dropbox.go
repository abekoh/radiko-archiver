package dropbox

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/abekoh/radiko-podcast/internal/config"
	sdk "github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/fsnotify/fsnotify"
)

func RunSyncer(ctx context.Context, cnf *config.Config) {
	logger := slog.Default().With("job", "dropbox-uploader")
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			panic(fmt.Errorf("failed to create watcher: %w", err))
		}
		defer watcher.Close()
		if err := watcher.Add(cnf.OutDirPath); err != nil {
			panic(fmt.Errorf("failed to add watcher: %w", err))
		}
		dbConfig := sdk.Config{
			Token: cnf.Dropbox.Token,
		}
		client := files.New(dbConfig)

		logger.Info("start watching")
		for {
			select {
			case event := <-watcher.Events:
				if event.Has(fsnotify.Create & fsnotify.Write & fsnotify.Remove) {
					sync(ctx, cnf, client, event.Name, event)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func sync(ctx context.Context, cnf *config.Config, client files.Client, path string, event fsnotify.Event) {
	logger := slog.Default().With("job", "dropbox-sync")

	logger.Debug("start sync", "op", event.Op, "path", path)

	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
		logger.Info("uploading", "path", path)

		f, err := os.Open(path)
		if err != nil {
			logger.Error("failed to open file", "error", err)
			return
		}
		defer f.Close()

		commitInfo := files.NewCommitInfo("/" + filepath.Base(path))
		commitInfo.Mode.Tag = "overwrite"
		_, err = client.Upload(&files.UploadArg{
			CommitInfo:  *commitInfo,
			ContentHash: "", // TODO: calculate hash
		}, f)
		if err != nil {
			logger.Error("failed to upload into dropbox", "error", err)
			return
		}

		logger.Info("uploaded", "path", path)
	} else if event.Has(fsnotify.Remove) {
		logger.Info("deleting", "path", path)

		_, err := client.DeleteV2(&files.DeleteArg{
			Path: "/" + filepath.Base(path),
		})
		if err != nil {
			logger.Error("failed to delete from dropbox", "error", err)
			return
		}

		logger.Info("deleted", "path", path)
	} else {
		logger.Error("unknown operation", "op", event.Op)
		return
	}

}
