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

	"github.com/abekoh/radiko-podcast/internal/config"
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

	var configPath, radikoTSURL string
	flag.StringVar(&configPath, "config", "config.toml", "config path")
	flag.StringVar(&radikoTSURL, "justnow", "", "fetch and encode just now with radiko time-shifted URL")
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
	if cnf.Server.Enabled {
		server.RunServer(ctx, cnf)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
