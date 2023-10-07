package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	OutDirPath string  `toml:"out_dir_path"`
	RulesPath  string  `toml:"rules_path"`
	Radiko     Radiko  `toml:"radiko"`
	Server     Server  `toml:"server"`
	Dropbox    Dropbox `toml:"dropbox"`
}

type Radiko struct {
	OffsetTimeStr      string `toml:"offset_time"`
	PlannerIntervalStr string `toml:"planner_interval"`
	FetchTimeoutStr    string `toml:"fetch_timeout"`

	OffsetTime      time.Duration `toml:"-"`
	PlannerInterval time.Duration `toml:"-"`
	FetchTimeout    time.Duration `toml:"-"`
}

type Server struct {
	Enabled bool   `toml:"enabled"`
	Port    int    `toml:"port"`
	BaseURL string `toml:"base_url"`
}

type Dropbox struct {
	Enabled bool   `toml:"enabled"`
	Token   string `toml:"-"`
}

func (r *Radiko) updateTime() error {
	offsetTime, err := time.ParseDuration(r.OffsetTimeStr)
	if err != nil {
		return fmt.Errorf("failed to parse offset_time: %w", err)
	}
	r.OffsetTime = offsetTime

	plannerInterval, err := time.ParseDuration(r.PlannerIntervalStr)
	if err != nil {
		return fmt.Errorf("failed to parse planner_interval: %w", err)
	}
	r.PlannerInterval = plannerInterval

	fetchTimeout, err := time.ParseDuration(r.FetchTimeoutStr)
	if err != nil {
		return fmt.Errorf("failed to parse fetch_timeout: %w", err)
	}
	r.FetchTimeout = fetchTimeout

	return nil
}

func Parse(path string) (*Config, error) {
	var cnf Config
	if _, err := toml.DecodeFile(path, &cnf); err != nil {
		return nil, err
	}
	if err := cnf.Radiko.updateTime(); err != nil {
		return nil, err
	}
	cnf.Dropbox.Token = os.Getenv("DROPBOX_TOKEN")
	return &cnf, nil
}
