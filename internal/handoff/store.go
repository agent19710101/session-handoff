package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storeLockTimeout  = 5 * time.Second
	storeLockPollWait = 50 * time.Millisecond
)

func loadStore() (storeFile, string, error) {
	path, err := defaultStorePath()
	if err != nil {
		return storeFile{}, "", err
	}
	s, err := loadStoreAtPath(path)
	if err != nil {
		return storeFile{}, "", err
	}
	return s, path, nil
}

func loadStoreAtPath(path string) (storeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storeFile{Version: 1, Items: []HandoffRecord{}}, nil
		}
		return storeFile{}, fmt.Errorf("read store: %w", err)
	}

	var s storeFile
	if err := json.Unmarshal(data, &s); err != nil {
		return storeFile{}, fmt.Errorf("parse store: %w", err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Items == nil {
		s.Items = []HandoffRecord{}
	}
	if err := validateStore(s); err != nil {
		return storeFile{}, err
	}
	return s, nil
}

func writeStore(path string, s storeFile) error {
	if err := validateStore(s); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".handoffs-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp store file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp store: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace store atomically: %w", err)
	}
	return nil
}

func updateStore(update func(*storeFile) error) error {
	path, err := defaultStorePath()
	if err != nil {
		return err
	}
	if err := withStoreLock(path, func() error {
		store, err := loadStoreAtPath(path)
		if err != nil {
			return err
		}
		if err := update(&store); err != nil {
			return err
		}
		return writeStore(path, store)
	}); err != nil {
		return err
	}
	return nil
}

func withStoreLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create store dir for lock: %w", err)
	}
	lockPath := path + ".lock"
	deadline := time.Now().Add(storeLockTimeout)

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString(fmt.Sprintf("pid=%d\n", os.Getpid()))
			_ = f.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acquire store lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("acquire store lock: timeout after %s", storeLockTimeout)
		}
		time.Sleep(storeLockPollWait)
	}
}

func validateStore(s storeFile) error {
	for i, rec := range s.Items {
		if err := validateRecord(rec); err != nil {
			return fmt.Errorf("invalid store record at index %d: %w", i, err)
		}
	}
	return nil
}

func validateRecord(rec HandoffRecord) error {
	if strings.TrimSpace(rec.CreatedAt) == "" {
		return errors.New("created_at is required")
	}
	ts, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("created_at must be RFC3339: %w", err)
	}
	if rec.CreatedAt != ts.UTC().Format(time.RFC3339) {
		return errors.New("created_at must be RFC3339 UTC (Z)")
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

func loadRecord(id string) (HandoffRecord, error) {
	store, _, err := loadStore()
	if err != nil {
		return HandoffRecord{}, err
	}
	if len(store.Items) == 0 {
		return HandoffRecord{}, errors.New("no handoffs saved")
	}
	return pickRecord(store.Items, id)
}
