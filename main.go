package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/google/go-cmp/cmp"
	"github.com/lmittmann/tint"
	"github.com/yyoshiki41/go-radiko"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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

func RunPlanner(ctx context.Context, toDispatcher chan<- []Schedule, rulesPath string) {
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

	go func() {
		loadr()
		updateSches()

		ticker := time.NewTicker(plannerInterval)
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			panic(fmt.Errorf("failed to create watcher: %w", err))
		}
		defer watcher.Close()
		if err := watcher.Add(rulesPath); err != nil {
			panic(fmt.Errorf("failed to add watcher: %w", err))
		}
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
	}()
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

func RunFetchers(ctx context.Context, toFetcher <-chan Schedule, outDirPath string) {
	logger := slog.Default().With("job", "fetchers")
	logger.Debug("start fetchers")

	radikoClient, err := radiko.New("")
	if err != nil {
		panic(fmt.Errorf("failed to create radiko client: %w", err))
	}
	_, err = radikoClient.AuthorizeToken(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to authorize token: %w", err))
	}

	go func() {
		for {
			select {
			case sche := <-toFetcher:
				c, cancel := context.WithTimeout(ctx, fetchTimeout)
				defer cancel()
				go func(ctx context.Context, s Schedule, log *slog.Logger) {
					log.Info("start fetching", "schedule", s)
					pg, err := radikoClient.GetProgramByStartTime(ctx, string(s.StationID), s.StartTime)
					if err != nil {
						log.Error("failed to fetch program", "error", err)
						return
					}
					log.Debug("get program", "program", pg)

					xmlFile, err := os.Create(fmt.Sprintf("%s/%s_%s_%s.xml", outDirPath, pg.Ft, s.StationID, pg.Title))
					if err != nil {
						log.Error("failed to create file", "error", err)
						return
					}
					defer xmlFile.Close()
					xmlEncoder := xml.NewEncoder(xmlFile)
					xmlEncoder.Indent("", "  ")
					if err := xmlEncoder.Encode(pg); err != nil {
						log.Error("failed to encode xml", "error", err)
						return
					}

					m3u8URI, err := radikoClient.TimeshiftPlaylistM3U8(ctx, string(s.StationID), s.StartTime)
					if err != nil {
						log.Error("failed to get m3u8URI", "error", err)
						return
					}
					log.Debug("got m3u8URI", "m3u8URI", m3u8URI)

					chunkList, err := radiko.GetChunklistFromM3U8(m3u8URI)
					if err != nil {
						log.Error("failed to get chunkList", "error", err)
						return
					}
					log.Debug("got chunkList", "chunkList[:5]", chunkList[:5], "len()", len(chunkList))

					tempDirPath := os.TempDir()
					log.Debug("tempDirPath", "tempDirPath", tempDirPath)
					if err := bulkDownload(
						ctx,
						chunkList,
						tempDirPath,
						string(s.StationID)+s.StartTime.Format("20060102150405")+"_",
					); err != nil {
						log.Error("failed to download chunks", "error", err)
						return
					}
					log.Debug("complete downloading chunks")

					log.Info("finish fetching")
				}(c, sche, logger.With("job", "fetcher-"+time.Now().Format("20060102150405")))
			case <-ctx.Done():
				logger.Debug("stop fetchers")
				return
			}
		}
	}()
}

const (
	maxAttempts    = 3
	maxConcurrents = 64
)

func bulkDownload(ctx context.Context, urls []string, outDirPath, fileNamePrefix string) error {
	sem := semaphore.NewWeighted(maxConcurrents)
	g, ctx := errgroup.WithContext(ctx)
	for _, url := range urls {
		url := url
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return fmt.Errorf("failed to acquire semaphore: %w", err)
			}
			defer sem.Release(1)

			attempts := 0
			for {
				attempts++
				_, urlFilename := filepath.Split(url)
				if err := download(ctx, url, outDirPath, fileNamePrefix+urlFilename); err != nil {
					if attempts >= maxAttempts {
						return fmt.Errorf("failed to download: %w", err)
					}
				} else {
					break
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to download one job: %w", err)
	}
	return nil
}

func download(ctx context.Context, url, outDirPath, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	file, err := os.Create(filepath.Join(outDirPath, filename))
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return err
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
	var rulesPath, outDirPath string
	flag.StringVar(&rulesPath, "rules", "rules.toml", "rules config path")
	flag.StringVar(&outDirPath, "out", "out", "output directory path")
	flag.Parse()

	logger := slog.Default().With("job", "main")

	toDispatcher := make(chan []Schedule)
	toFetcher := make(chan Schedule)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RunPlanner(ctx, toDispatcher, rulesPath)
	RunDispatcher(ctx, toDispatcher, toFetcher)
	RunFetchers(ctx, toFetcher, outDirPath)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	logger.Info("received SIGTERM")
}
