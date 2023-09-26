package main

import (
	"context"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/lmittmann/tint"
)

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type StationID string

const (
	TBS StationID = "TBS" // TBSラジオ
	LFR StationID = "LFR" // ニッポン放送
)

const (
	offsetTime      = 6 * time.Hour
	plannerInterval = 10 * time.Minute
)

type Rule struct {
	Name        string
	StationID   StationID
	Weekday     time.Weekday
	StartHour   int
	StartMinute int
	Duration    time.Duration
}

func (r Rule) NextSchedules(n int) []Schedule {
	if n <= 0 {
		return []Schedule{}
	}
	schedules := make([]Schedule, n)
	currentTime := time.Now().Add(-offsetTime)
	for i := 0; i < n; i++ {
		schedules[i] = r.nextSchedule(currentTime)
		currentTime = schedules[i].StartTime
	}
	return schedules
}

func (r Rule) nextSchedule(t time.Time) Schedule {
	dayAbs := t.Weekday() - r.Weekday
	if dayAbs < 0 {
		dayAbs += 7
	}
	s := Schedule{
		StationID: r.StationID,
		StartTime: time.Date(t.Year(), t.Month(), t.Day()-int(dayAbs), r.StartHour, r.StartMinute, 0, 0, JST),
		Duration:  r.Duration,
	}
	if s.StartTime.Before(t) || s.StartTime.Equal(t) {
		s.StartTime = s.StartTime.AddDate(0, 0, 7)
	}
	s.FetchTime = s.StartTime.Add(offsetTime)
	return s
}

type Schedule struct {
	StationID StationID
	StartTime time.Time
	Duration  time.Duration
	FetchTime time.Time
}

var rules []Rule = []Rule{
	{
		Name:        "星野源のオールナイトニッポン",
		StationID:   LFR,
		Weekday:     time.Wednesday,
		StartHour:   1,
		StartMinute: 0,
		Duration:    2 * time.Hour,
	},
	{
		Name:        "オードリーのオールナイトニッポン",
		StationID:   LFR,
		Weekday:     time.Tuesday,
		StartHour:   16,
		StartMinute: 12,
		Duration:    2 * time.Hour,
	},
}

func newSchedules() []Schedule {
	newSches := make([]Schedule, 0, 100)
	for _, rule := range rules {
		newSches = append(newSches, rule.NextSchedules(3)...)
	}
	slices.SortFunc(newSches, func(a, b Schedule) int {
		if a.StartTime.Before(b.StartTime) {
			return -1
		} else if a.StartTime.After(b.StartTime) {
			return 1
		}
		return 0
	})
	return newSches
}

func runPlanner(toDispatcher chan<- []Schedule) {
	logger := slog.Default().With("job", "planner")
	logger.Debug("start planner")
	sches := newSchedules()
	toDispatcher <- sches

	ticker := time.NewTicker(plannerInterval)
	for range ticker.C {
		logger.Debug("update schedules")
		newSches := newSchedules()
		if diff := cmp.Diff(sches, newSches); diff != "" {
			logger.Info("schedules updated", "diff", diff)
			sches = newSches
			toDispatcher <- sches
		}
	}
}

func runDispatcher(toDispatcher <-chan []Schedule, toFetcher chan<- Schedule) {
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
	ticker := time.NewTicker(nextDispatchDuration())

	for {
		select {
		case <-ticker.C:
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
			ticker.Reset(nextDispatchDuration())
		case sches = <-toDispatcher:
			logger.Debug("receive new schedules", "schedules", sches)
			ticker.Reset(nextDispatchDuration())
		}
	}
}

func runFetchers(toFetcher <-chan Schedule) {
	logger := slog.Default().With("job", "fetchers")
	logger.Debug("start fetchers")
	for {
		select {
		case sche := <-toFetcher:
			go func(ctx context.Context, s Schedule, log *slog.Logger) {
				log.Info("start fetching", "schedule", s)
				time.Sleep(5 * time.Second)
				log.Info("finish fetching", "schedule", s)
			}(context.TODO(), sche, logger.With("job", "fetcher-"+time.Now().Format("20060102150405")))
		}
	}
}

func main() {
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

	logger := slog.Default().With("job", "main")

	toDispatcher := make(chan []Schedule)
	toFetcher := make(chan Schedule)

	go runPlanner(toDispatcher)
	go runDispatcher(toDispatcher, toFetcher)
	go runFetchers(toFetcher)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
