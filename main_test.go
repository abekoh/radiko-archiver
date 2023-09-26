package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunFetchers(t *testing.T) {
	tmpDir := t.TempDir()
	toFetcher := make(chan Schedule)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunFetchers(ctx, toFetcher, tmpDir)
	toFetcher <- Schedule{
		RuleName:  "オードリーのオールナイトニッポン",
		StationID: LFR,
		StartTime: time.Date(2023, 9, 24, 1, 0, 0, 0, JST),
		Duration:  2 * time.Hour,
		FetchTime: time.Date(2023, 9, 24, 7, 0, 0, 0, JST),
	}
	time.Sleep(3 * time.Second)

	xmlPath := filepath.Join(tmpDir, "20230924010000_LFR_オードリーのオールナイトニッポン.xml")
	if _, err := os.Stat(xmlPath); errors.Is(err, os.ErrNotExist) {
		t.Errorf("file does not exist: %v", err)
	}
	xmlFile, err := os.ReadFile(xmlPath)
	if err != nil {
		t.Errorf("failed to read file: %v", err)
	}
	t.Logf("file content: %s\n", xmlFile)
}
