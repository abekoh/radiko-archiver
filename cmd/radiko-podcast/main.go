package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abekoh/radiko-podcast/internal/radiko"
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
	var rulesPath, outDirPath string
	flag.StringVar(&rulesPath, "rules", "rules.toml", "rules config path")
	flag.StringVar(&outDirPath, "out", "out", "output directory path")
	flag.Parse()

	radiko.Run(context.Background(), rulesPath, outDirPath)

	logger := slog.Default().With("job", "main")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
