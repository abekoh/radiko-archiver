package radiko

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"
)

func RunScheduler(ctx context.Context, rulesPath, outDirPath string) {
	toDispatcher := make(chan []Schedule)
	toFetcher := make(chan Schedule)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RunPlanner(ctx, toDispatcher, rulesPath)
	RunDispatcher(ctx, toDispatcher, toFetcher)
	RunFetchers(ctx, toFetcher, outDirPath, nil)
}

func RunFromURL(ctx context.Context, tsURL, outDirPath string) {
	logger := slog.Default().With("job", "run-from-url")
	toFetcher := make(chan Schedule)
	toDone := make(chan bool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sche, err := parseURL(tsURL)
	if err != nil {
		logger.Error("failed to parse URL", "error", err)
		return
	}
	logger.Info("start", "schedule", sche)
	RunFetchers(ctx, toFetcher, outDirPath, toDone)
	toFetcher <- sche
	if <-toDone {
		logger.Info("done")
	} else {
		logger.Error("failed")
	}
}

func parseURL(tsURL string) (Schedule, error) {
	re := regexp.MustCompile(`\/ts\/([A-Z]+)\/([0-9]+)$`)
	matches := re.FindStringSubmatch(tsURL)
	if len(matches) < 3 {
		return Schedule{}, fmt.Errorf("invalid URL format")
	}

	stationID := matches[1]
	startTimeStr := matches[2]

	startTime, err := time.ParseInLocation("20060102150405", startTimeStr, JST)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{
		RuleName:  "FromURL",
		StationID: StationID(stationID),
		StartTime: startTime,
		FetchTime: time.Now(),
	}, nil
}
