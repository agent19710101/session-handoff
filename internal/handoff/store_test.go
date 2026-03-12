package handoff

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadStoreRejectsNonUTCRecordTimestamp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	store := storeFile{Version: 1, Items: []HandoffRecord{{
		ID:        "a",
		CreatedAt: "2026-03-12T08:00:00+01:00",
		Tool:      "codex",
		Project:   "/tmp/p",
		Title:     "t",
		Summary:   "s",
	}}}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}

	if _, _, err := loadStore(); err == nil {
		t.Fatalf("expected loadStore to reject non-UTC created_at")
	}
}

func TestImportRejectsNonUTCTimestamp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	bundle := ExportBundle{
		Version: 2,
		Record: HandoffRecord{
			ID:        "x1",
			CreatedAt: "2026-03-12T08:00:00+01:00",
			Tool:      "codex",
			Project:   "/tmp/p",
			Title:     "bad ts",
			Summary:   "should fail",
		},
	}
	digest, err := RecordChecksum(bundle.Record)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	bundle.Checksum = digest
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	input := filepath.Join(tmp, "bundle.json")
	if err := os.WriteFile(input, data, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	if err := cmdImport([]string{"--input", input}, io.Discard); err == nil {
		t.Fatalf("expected import to reject non-UTC timestamp")
	}
}

func TestConcurrentSavesDoNotLoseRecords(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			errCh <- cmdSave([]string{
				"--tool", "codex",
				"--project", tmp,
				"--title", "t",
				"--summary", "s",
				"--next", "n",
			}, io.Discard)
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("cmdSave failed: %v", err)
		}
	}

	store, _, err := loadStore()
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(store.Items) != workers {
		t.Fatalf("expected %d records, got %d", workers, len(store.Items))
	}
}
