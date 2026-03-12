package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func loadStore() (storeFile, string, error) {
	path, err := defaultStorePath()
	if err != nil {
		return storeFile{}, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storeFile{Version: 1, Items: []handoffRecord{}}, path, nil
		}
		return storeFile{}, "", fmt.Errorf("read store: %w", err)
	}

	var s storeFile
	if err := json.Unmarshal(data, &s); err != nil {
		return storeFile{}, "", fmt.Errorf("parse store: %w", err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Items == nil {
		s.Items = []handoffRecord{}
	}
	return s, path, nil
}

func writeStore(path string, s storeFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func defaultStorePath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(cfg, "session-handoff", "handoffs.json"), nil
}

func loadRecord(id string) (handoffRecord, error) {
	store, _, err := loadStore()
	if err != nil {
		return handoffRecord{}, err
	}
	if len(store.Items) == 0 {
		return handoffRecord{}, errors.New("no handoffs saved")
	}
	return pickRecord(store.Items, id)
}
