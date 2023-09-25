package main

import (
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
)

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type StationID string

const (
	TBS StationID = "TBS" // TBSラジオ
	LFR StationID = "LFR" // ニッポン放送
)

const offsetTime = 6 * time.Hour

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
		Weekday:     time.Monday,
		StartHour:   19,
		StartMinute: 27,
		Duration:    2 * time.Hour,
	},
}

var (
	schedules        = make([]Schedule, 0, 100)
	schedulesMu      sync.RWMutex
	scheduleUpdateCh = make(chan struct{})
)

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

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	go func() {
		schedulesMu.Lock()
		schedules = newSchedules()
		logger.Info("default schedules", "schedule", schedules)
		schedulesMu.Unlock()

		ticker := time.NewTicker(10 * time.Minute)
		logger.Debug("start updateSchedule ticker")
		for range ticker.C {
			logger.Debug("update schedules")
			newSches := newSchedules()
			if diff := cmp.Diff(schedules, newSches); diff != "" {
				logger.Info("schedules updated", "diff", diff)
				schedulesMu.Lock()
				schedules = newSches
				schedulesMu.Unlock()
				scheduleUpdateCh <- struct{}{}
			}
		}
	}()

	go func() {
		schedulesMu.RLock()
		ticker := time.NewTicker(min(1*time.Minute, schedules[0].FetchTime.Sub(time.Now())))
		logger.Debug("start scheduler ticker")
		schedulesMu.RUnlock()

		for {
			select {
			case <-ticker.C:
				logger.Debug("ticker fired")
				schedulesMu.RLock()
				for _, sche := range schedules {
					if sche.FetchTime.Before(time.Now()) {
						logger.Info("start fetching", "schedule", sche)
					} else {
						ticker.Reset(min(1*time.Minute, sche.FetchTime.Sub(time.Now())))
						break
					}
				}
				schedulesMu.RUnlock()
			case <-scheduleUpdateCh:
				logger.Debug("receive schedule update")
				schedulesMu.RLock()
				nextFetchTime := schedules[0].FetchTime
				schedulesMu.RUnlock()
				ticker.Reset(min(1*time.Minute, nextFetchTime.Sub(time.Now())))
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
