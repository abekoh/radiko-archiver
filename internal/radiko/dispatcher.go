package radiko

import (
	"context"
	"log/slog"
	"math"
	"time"
)

func RunDispatcher(ctx context.Context, toDispatcher <-chan []Schedule, toFetcher chan<- Schedule) {
	logger := slog.Default().With("job", "dispatcher")
	logger.Debug("start dispatcher")
	sches := <-toDispatcher
	nextDispatchDuration := func() time.Duration {
		if len(sches) > 0 {
			return sches[0].FetchTime.Sub(time.Now())
		} else {
			return math.MaxInt64
		}
	}

	go func() {
		timer := time.NewTimer(nextDispatchDuration())
		for {
			select {
			case <-timer.C:
				logger.Debug("dispatch start")
				for len(sches) > 0 {
					if sches[0].FetchTime.Before(time.Now()) || sches[0].FetchTime.Equal(time.Now()) {
						logger.Debug("dispatch", "schedule", sches[0])
						toFetcher <- sches[0]
						sches = sches[1:]
					} else {
						break
					}
				}
				timer.Reset(nextDispatchDuration())
			case sches = <-toDispatcher:
				logger.Debug("receive new schedules", "schedules", sches)
				timer.Reset(nextDispatchDuration())
			case <-ctx.Done():
				logger.Debug("stop dispatcher")
				return
			}
		}
	}()
}
