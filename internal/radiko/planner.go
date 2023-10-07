package radiko

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/abekoh/radiko-podcast/internal/config"
	"github.com/fsnotify/fsnotify"
	"github.com/google/go-cmp/cmp"
)

func RunPlanner(ctx context.Context, toDispatcher chan<- []Schedule, cnf *config.Config) {
	logger := slog.Default().With("job", "planner")
	logger.Debug("start planner")

	var rules []Rule
	loadr := func() bool {
		rs, err := loadRules(cnf.RulesPath)
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
		if err := watcher.Add(cnf.RulesPath); err != nil {
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
