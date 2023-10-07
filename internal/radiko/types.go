package radiko

import (
	"fmt"
	"slices"
	"time"

	"github.com/BurntSushi/toml"
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
	FetchTime time.Time
}

func (s Schedule) String() string {
	return fmt.Sprintf(
		"[%s] %s %s(fetchTime:%s)",
		s.StationID,
		s.RuleName,
		s.StartTime.Format("2006/01/02 15:04"),
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
