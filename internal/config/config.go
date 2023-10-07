package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	OutDirPath string `toml:"out_dir_path"`
	RulesPath  string `toml:"rules_path"`
	Radiko     Radiko `toml:"radiko"`
	Server     Server `toml:"server"`
}

type Radiko struct {
	offsetTimeStr      string `toml:"offset_time"`
	plannerIntervalStr string `toml:"planner_interval"`
	fetchTimeoutStr    string `toml:"fetch_timeout"`

	OffsetTime      time.Duration `toml:"-"`
	PlannerInterval time.Duration `toml:"-"`
	FetchTimeout    time.Duration `toml:"-"`
}

func (r *Radiko) updateTime() error {
	offsetTime, err := time.ParseDuration(r.offsetTimeStr)
	if err != nil {
		return err
	}
	r.OffsetTime = offsetTime

	plannerInterval, err := time.ParseDuration(r.plannerIntervalStr)
	if err != nil {
		return err
	}
	r.PlannerInterval = plannerInterval

	fetchTimeout, err := time.ParseDuration(r.fetchTimeoutStr)
	if err != nil {
		return err
	}
	r.FetchTimeout = fetchTimeout

	return nil
}

type Server struct {
	BaseURL string `toml:"base_url"`
}

func Parse(path string) (*Config, error) {
	var cnf Config
	if _, err := toml.DecodeFile(path, &cnf); err != nil {
		return nil, err
	}
	if err := cnf.Radiko.updateTime(); err != nil {
		return nil, err
	}
	return &cnf, nil
}
