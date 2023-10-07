package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abekoh/radiko-podcast/internal/config"
	"github.com/abekoh/radiko-podcast/internal/dropbox"
	"github.com/abekoh/radiko-podcast/internal/feed"
	"github.com/abekoh/radiko-podcast/internal/radiko"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func init() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	var logLevel slog.Level
	logLevelEnv := os.Getenv("LOG_LEVEL")
	switch {
	case strings.EqualFold(logLevelEnv, "debug"):
		logLevel = slog.LevelDebug
	case strings.EqualFold(logLevelEnv, "warn"):
		logLevel = slog.LevelWarn
	case strings.EqualFold(logLevelEnv, "error"):
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logColorEnv := os.Getenv("LOG_COLOR")
	if strings.EqualFold(logColorEnv, "true") {
		slog.SetDefault(
			slog.New(
				tint.NewHandler(
					os.Stderr,
					&tint.Options{
						Level:      logLevel,
						TimeFormat: time.Kitchen,
					},
				),
			),
		)
	}

}

func main() {
	logger := slog.Default().With("job", "main")

	var configPath, radikoTSURL string
	flag.StringVar(&configPath, "config", "config.toml", "config path")
	flag.StringVar(&radikoTSURL, "now", "", "fetch and encode just now with radiko time-shifted URL")
	flag.Parse()

	cnf, err := config.Parse(configPath)
	if err != nil {
		logger.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cnf.OutDirPath, 0755); err != nil {
		logger.Error("failed to create output directory", "error", err)
		os.Exit(1)
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		logger.Error("ffmpeg command is not available", "error", err)
		os.Exit(1)
	}

	if radikoTSURL != "" {
		radiko.RunFromURL(context.Background(), radikoTSURL, cnf)
		return
	}

	ctx := context.Background()

	radiko.RunScheduler(ctx, cnf)
	if cnf.Feed.Enabled {
		feed.RunServer(ctx, cnf)
	}
	if cnf.Dropbox.Enabled {
		dropbox.RunSyncer(ctx, cnf)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
