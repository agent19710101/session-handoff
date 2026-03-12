package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportJSONAndImport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Add parser",
		"--summary", "Implemented parser traversal",
		"--next", "Add fixtures",
	}); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath}); err != nil {
		t.Fatalf("cmdExport json failed: %v", err)
	}

	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("bundle file not created: %v", err)
	}

	// reset store to simulate transfer/import on another machine
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := os.Remove(storePath); err != nil {
		t.Fatalf("remove store: %v", err)
	}

	if err := cmdImport([]string{"--input", bundlePath}); err != nil {
		t.Fatalf("cmdImport failed: %v", err)
	}

	rec, err := loadRecord("latest")
	if err != nil {
		t.Fatalf("loadRecord latest failed: %v", err)
	}
	if rec.Title != "Add parser" {
		t.Fatalf("unexpected imported title: %q", rec.Title)
	}
}

func TestListJSONEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	callErr := cmdList([]string{"--json"})
	_ = w.Close()
	os.Stdout = origStdout
	if callErr != nil {
		t.Fatalf("cmdList failed: %v", callErr)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}
