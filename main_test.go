package main

import (
	"bytes"
	"encoding/json"
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

func TestExportJSONIncludesChecksum(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Checksum coverage",
		"--summary", "Validating bundle integrity",
	}); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath}); err != nil {
		t.Fatalf("cmdExport json failed: %v", err)
	}

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}

	var bundle exportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	if bundle.Version != 2 {
		t.Fatalf("unexpected version: %d", bundle.Version)
	}
	if strings.TrimSpace(bundle.Checksum) == "" {
		t.Fatalf("checksum should be present")
	}
}

func TestImportRejectsChecksumMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	rec := handoffRecord{
		ID:        "20260312-020000",
		CreatedAt: "2026-03-12T01:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/project",
		Title:     "Broken bundle",
		Summary:   "Checksum mismatch test",
	}

	bundle := exportBundle{
		Version:  2,
		Checksum: strings.Repeat("0", 64),
		Record:   rec,
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bad-bundle.json")
	if err := os.WriteFile(bundlePath, data, 0o644); err != nil {
		t.Fatalf("write bad bundle: %v", err)
	}

	err = cmdImport([]string{"--input", bundlePath})
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
