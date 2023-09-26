package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
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
	rulesPath       = "rules.toml"
	fetchTimeout    = 3 * time.Minute
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
		RuleName:  r.Name,
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
	RuleName  string
	StationID StationID
	StartTime time.Time
	Duration  time.Duration
	FetchTime time.Time
}

func (s Schedule) String() string {
	return fmt.Sprintf(
		"[%s] %s %s-%s(%s)(fetchTime:%s)",
		s.StationID,
		s.RuleName,
		s.StartTime.Format("2006/01/02 15:04"),
		s.StartTime.Add(s.Duration).Format("15:04"),
		s.Duration,
		s.FetchTime.Format("2006/01/02 15:04"),
	)
}

func loadRules(path string) ([]Rule, error) {
	type tomlConfig struct {
		Rules []struct {
			Name      string `toml:"name"`
			StationID string `toml:"station_id"`
			Weekday   string `toml:"weekday"`
			Start     string `toml:"start"`
			Duration  string `toml:"duration"`
		} `toml:"rules"`
	}
	var config tomlConfig
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	rules := make([]Rule, len(config.Rules))
	for i, cRule := range config.Rules {
		rules[i] = Rule{
			Name:      cRule.Name,
			StationID: StationID(cRule.StationID),
		}
		switch cRule.Weekday {
		case "Sun":
			rules[i].Weekday = time.Sunday
		case "Mon":
			rules[i].Weekday = time.Monday
		case "Tue":
			rules[i].Weekday = time.Tuesday
		case "Wed":
			rules[i].Weekday = time.Wednesday
		case "Thu":
			rules[i].Weekday = time.Thursday
		case "Fri":
			rules[i].Weekday = time.Friday
		case "Sat":
			rules[i].Weekday = time.Saturday
		default:
			return nil, fmt.Errorf("invalid weekday: %s", cRule.Weekday)
		}
		if _, err := fmt.Sscanf(cRule.Start, "%d:%d", &rules[i].StartHour, &rules[i].StartMinute); err != nil {
			return nil, fmt.Errorf("invalid start time: %s", cRule.Start)
		}
		dur, err := time.ParseDuration(cRule.Duration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %s", cRule.Duration)
		}
		rules[i].Duration = dur
	}
	return rules, nil
}

func newSchedules(rules []Rule) []Schedule {
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

func RunPlanner(ctx context.Context, toDispatcher chan<- []Schedule) {
	logger := slog.Default().With("job", "planner")
	logger.Debug("start planner")

	var rules []Rule
	loadr := func() bool {
		rs, err := loadRules(rulesPath)
		if err != nil {
			logger.Error("failed to load rules", "error", err)
			return false
		}
		rules = rs
		logger.Debug("load rules", "rules", rules)
		return true
	}
	loadr()

	var sches []Schedule
	updateSches := func() {
		logger.Debug("update schedules")
		newSches := newSchedules(rules)
		if diff := cmp.Diff(sches, newSches); diff != "" {
			logger.Info("schedules updated", "new", newSches)
			sches = newSches
			toDispatcher <- sches
		}
	}
	updateSches()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(fmt.Errorf("failed to create watcher: %w", err))
	}
	defer watcher.Close()
	if err := watcher.Add(rulesPath); err != nil {
		panic(fmt.Errorf("failed to add watcher: %w", err))
	}

	ticker := time.NewTicker(plannerInterval)
	for {
		select {
		case <-ticker.C:
			updateSches()
		case event := <-watcher.Events:
			if event.Has(fsnotify.Write) {
				logger.Debug("rules file updated", "path", event.Name)
				if loadr() {
					updateSches()
				}
			}
		case <-ctx.Done():
			logger.Debug("stop planner")
			return
		}
	}
}

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
}

func RunFetchers(ctx context.Context, toFetcher <-chan Schedule) {
	logger := slog.Default().With("job", "fetchers")
	logger.Debug("start fetchers")
	for {
		select {
		case sche := <-toFetcher:
			c, cancel := context.WithTimeout(ctx, fetchTimeout)
			defer cancel()
			go func(ctx context.Context, s Schedule, log *slog.Logger) {
				log.Info("start fetching", "schedule", s)
				time.Sleep(5 * time.Second)
				log.Info("finish fetching", "schedule", s)
			}(c, sche, logger.With("job", "fetcher-"+time.Now().Format("20060102150405")))
		case <-ctx.Done():
			logger.Debug("stop fetchers")
			return
		}
	}
}

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

	toDispatcher := make(chan []Schedule)
	toFetcher := make(chan Schedule)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go RunPlanner(ctx, toDispatcher)
	go RunDispatcher(ctx, toDispatcher, toFetcher)
	go RunFetchers(ctx, toFetcher)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
