package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/abekoh/radiko-podcast/internal/radiko"
	"github.com/abekoh/radiko-podcast/internal/server"
	"github.com/lmittmann/tint"
)

func init() {
	slog.SetDefault(
		slog.New(
			tint.NewHandler(
				os.Stderr,
				&tint.Options{
					Level:      slog.LevelDebug,
					TimeFormat: time.Kitchen,
				},
			),
		),
	)
}

func main() {
	logger := slog.Default().With("job", "main")

	var rulesPath, outDirPath, url, baseURL string
	flag.StringVar(&rulesPath, "rules", "rules.toml", "rules config path")
	flag.StringVar(&outDirPath, "out", "out", "output directory path")
	flag.StringVar(&baseURL, "baseurl", "http://localhost:8080", "base url")
	flag.StringVar(&url, "url", "", "radiko channel url")
	flag.Parse()

	if err := os.MkdirAll(outDirPath, 0755); err != nil {
		logger.Error("failed to create output directory", "error", err)
		os.Exit(1)
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		logger.Error("ffmpeg command is not available", "error", err)
		os.Exit(1)
	}

	if url != "" {
		radiko.RunFromURL(context.Background(), url, outDirPath)
		return
	}

	ctx := context.Background()

	radiko.RunScheduler(ctx, rulesPath, outDirPath)
	server.RunServer(ctx, outDirPath, baseURL)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
