package handoff

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportJSONAndImport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	initGitRepo(t, tmp)

	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Add parser",
		"--summary", "Implemented parser traversal",
		"--next", "Add fixtures",
	}, io.Discard); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath}, io.Discard); err != nil {
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

	if err := cmdImport([]string{"--input", bundlePath, "--allow-unsigned"}, io.Discard); err != nil {
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

	callErr := cmdList([]string{"--json"}, w)
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
	initGitRepo(t, tmp)

	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Checksum coverage",
		"--summary", "Validating bundle integrity",
	}, io.Discard); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath}, io.Discard); err != nil {
		t.Fatalf("cmdExport json failed: %v", err)
	}

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}

	var bundle ExportBundle
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

	rec := HandoffRecord{
		ID:        "20260312-020000",
		CreatedAt: "2026-03-12T01:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/project",
		Title:     "Broken bundle",
		Summary:   "Checksum mismatch test",
	}

	bundle := ExportBundle{
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

	err = cmdImport([]string{"--input", bundlePath, "--allow-unsigned"}, io.Discard)
	if err == nil {
		t.Fatalf("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListFiltersByToolAndLimit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "a", CreatedAt: time.Date(2026, 3, 12, 1, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "A", Summary: "s"},
			{ID: "b", CreatedAt: time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "claude-code", Project: "/p", Title: "B", Summary: "s"},
			{ID: "c", CreatedAt: time.Date(2026, 3, 12, 3, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "C", Summary: "s"},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	callErr := cmdList([]string{"--json", "--tool", "codex", "--limit", "1"}, w)
	_ = w.Close()
	os.Stdout = origStdout
	if callErr != nil {
		t.Fatalf("cmdList failed: %v", callErr)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read output: %v", err)
	}

	var got []HandoffRecord
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output=%s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].ID != "c" {
		t.Fatalf("expected latest codex record c, got %q", got[0].ID)
	}
}

func TestListFiltersByIDPrefix(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "20260312-010000", CreatedAt: "2026-03-12T01:00:00Z", Tool: "codex", Project: "/p", Title: "A", Summary: "s"},
			{ID: "20260312-020000", CreatedAt: "2026-03-12T02:00:00Z", Tool: "codex", Project: "/p", Title: "B", Summary: "s"},
			{ID: "20260311-230000", CreatedAt: "2026-03-11T23:00:00Z", Tool: "codex", Project: "/p", Title: "C", Summary: "s"},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out bytes.Buffer
	if err := cmdList([]string{"--json", "--id", "20260312"}, &out); err != nil {
		t.Fatalf("cmdList failed: %v", err)
	}

	var got []HandoffRecord
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output=%s", err, out.String())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records for prefix, got %d", len(got))
	}
	if got[0].ID != "20260312-020000" || got[1].ID != "20260312-010000" {
		t.Fatalf("unexpected id order: %+v", got)
	}
}

func TestListRejectsNegativeLimit(t *testing.T) {
	if err := cmdList([]string{"--limit", "-1"}, io.Discard); err == nil {
		t.Fatalf("expected validation error for negative limit")
	}
}

func TestListRejectsLatestWithLimit(t *testing.T) {
	if err := cmdList([]string{"--latest", "--limit", "1"}, io.Discard); err == nil {
		t.Fatalf("expected validation error for --latest with --limit")
	}
}

func TestListLatestReturnsSingleMostRecentRecord(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "a", CreatedAt: time.Date(2026, 3, 12, 1, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "A", Summary: "s"},
			{ID: "b", CreatedAt: time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "B", Summary: "s"},
			{ID: "c", CreatedAt: time.Date(2026, 3, 12, 3, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "C", Summary: "s"},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out bytes.Buffer
	if err := cmdList([]string{"--json", "--latest"}, &out); err != nil {
		t.Fatalf("cmdList failed: %v", err)
	}

	var got []HandoffRecord
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output=%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].ID != "c" {
		t.Fatalf("expected most-recent record c, got %q", got[0].ID)
	}
}

func TestListRejectsInvalidSince(t *testing.T) {
	if err := cmdList([]string{"--since", "later"}, io.Discard); err == nil {
		t.Fatalf("expected validation error for invalid --since")
	}
}

func TestListFiltersByQuery(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "a", CreatedAt: time.Date(2026, 3, 12, 1, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "Refactor parser", Summary: "Parser cleanup", Next: []string{"Add tests"}},
			{ID: "b", CreatedAt: time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: "/p", Title: "Update docs", Summary: "README examples", Next: []string{"Publish release"}},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out bytes.Buffer
	if err := cmdList([]string{"--json", "--query", "parser"}, &out); err != nil {
		t.Fatalf("cmdList failed: %v", err)
	}

	var got []HandoffRecord
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output=%s", err, out.String())
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("expected only parser record a, got %+v", got)
	}
}

func TestFilterBySince(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	items := []HandoffRecord{
		{ID: "old", CreatedAt: now.Add(-3 * time.Hour).Format(time.RFC3339)},
		{ID: "keep", CreatedAt: now.Add(-45 * time.Minute).Format(time.RFC3339)},
		{ID: "future", CreatedAt: now.Add(15 * time.Minute).Format(time.RFC3339)},
		{ID: "bad", CreatedAt: "not-a-time"},
	}

	got := filterBySince(items, now, 90*time.Minute)
	if len(got) != 2 {
		t.Fatalf("expected 2 recent records, got %d", len(got))
	}
	if got[0].ID != "keep" || got[1].ID != "future" {
		t.Fatalf("unexpected records after since filter: %+v", got)
	}
}

func TestListFiltersByProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	p1 := filepath.Join(tmp, "one")
	p2 := filepath.Join(tmp, "two")
	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "a", CreatedAt: time.Date(2026, 3, 12, 1, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: p1, Title: "A", Summary: "s"},
			{ID: "b", CreatedAt: time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC).Format(time.RFC3339), Tool: "codex", Project: p2, Title: "B", Summary: "s"},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	callErr := cmdList([]string{"--json", "--project", p2}, w)
	_ = w.Close()
	os.Stdout = origStdout
	if callErr != nil {
		t.Fatalf("cmdList failed: %v", callErr)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read output: %v", err)
	}

	var got []HandoffRecord
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v; output=%s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}
	if got[0].ID != "b" {
		t.Fatalf("expected project-filtered record b, got %q", got[0].ID)
	}
}

func TestCmdSaveRequiredFlagsTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cases := []struct {
		name string
		args []string
	}{
		{name: "missing tool", args: []string{"--project", tmp, "--title", "t", "--summary", "s"}},
		{name: "missing project", args: []string{"--tool", "codex", "--title", "t", "--summary", "s"}},
		{name: "missing title", args: []string{"--tool", "codex", "--project", tmp, "--summary", "s"}},
		{name: "missing summary", args: []string{"--tool", "codex", "--project", tmp, "--title", "t"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdSave(tc.args, io.Discard)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), "save requires --tool, --project, --title, --summary") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCmdSaveNormalizesProjectPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cwd := filepath.Join(tmp, "workspace")
	project := filepath.Join(cwd, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	initGitRepo(t, project)

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prev) }()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", "./project",
		"--title", "Path normalize",
		"--summary", "Ensure project path becomes absolute",
	}, io.Discard); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	rec, err := loadRecord("latest")
	if err != nil {
		t.Fatalf("load latest: %v", err)
	}
	if rec.Project != project {
		t.Fatalf("expected absolute project path %q, got %q", project, rec.Project)
	}
}

func TestCmdRenderOutputSections(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")

	withSignals := HandoffRecord{
		ID:        "with-signals",
		CreatedAt: "2026-03-12T05:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/proj",
		Title:     "A",
		Summary:   "Current summary",
		Changed:   []string{"M cmd/main.go", "?? notes.md"},
		Next:      []string{"Run tests"},
	}
	withoutSignals := withSignals
	withoutSignals.ID = "without-signals"
	withoutSignals.Changed = nil

	if err := writeStore(storePath, storeFile{Version: 1, Items: []HandoffRecord{withSignals, withoutSignals}}); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out bytes.Buffer
	if err := cmdRender([]string{"--id", "with-signals", "--target", "claude-code"}, &out); err != nil {
		t.Fatalf("cmdRender with signals failed: %v", err)
	}
	rendered := out.String()
	for _, want := range []string{"## Current state", "## Constraints", "## Working tree signals", "M cmd/main.go", "Target tool: claude-code"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("render output missing %q:\n%s", want, rendered)
		}
	}

	out.Reset()
	if err := cmdRender([]string{"--id", "without-signals", "--target", "claude-code"}, &out); err != nil {
		t.Fatalf("cmdRender without signals failed: %v", err)
	}
	if strings.Contains(out.String(), "## Working tree signals") {
		t.Fatalf("did not expect working tree section when changed is empty:\n%s", out.String())
	}
}

func TestCmdExportUnsupportedFormat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	initGitRepo(t, tmp)
	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Export format",
		"--summary", "Check unsupported format",
	}, io.Discard); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	err := cmdExport([]string{"--id", "latest", "--format", "xml"}, io.Discard)
	if err == nil {
		t.Fatalf("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "unsupported export format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdExportStdoutAndFileModes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	initGitRepo(t, tmp)
	if err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Export modes",
		"--summary", "Validate stdout and file modes",
	}, io.Discard); err != nil {
		t.Fatalf("cmdSave failed: %v", err)
	}

	var out bytes.Buffer
	if err := cmdExport([]string{"--id", "latest", "--format", "markdown", "--target", "claude-code"}, &out); err != nil {
		t.Fatalf("cmdExport stdout failed: %v", err)
	}
	if !strings.Contains(out.String(), "# Session Handoff") {
		t.Fatalf("expected markdown in stdout, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Target tool: claude-code") {
		t.Fatalf("expected markdown target context, got: %s", out.String())
	}

	path := filepath.Join(tmp, "handoff.md")
	out.Reset()
	if err := cmdExport([]string{"--id", "latest", "--format", "markdown", "--target", "", "--output", path}, &out); err != nil {
		t.Fatalf("cmdExport file failed: %v", err)
	}
	if !strings.Contains(out.String(), "exported "+path) {
		t.Fatalf("expected exported message, got %q", out.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if !strings.Contains(string(data), "## Requested continuation") {
		t.Fatalf("unexpected file content: %s", string(data))
	}
	if !strings.Contains(string(data), "Target tool: generic") {
		t.Fatalf("expected fallback generic target, got: %s", string(data))
	}
}

func TestCmdRenderDefaultsTargetToGeneric(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")

	rec := HandoffRecord{
		ID:        "latest-like",
		CreatedAt: "2026-03-12T10:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/proj",
		Title:     "Render default target",
		Summary:   "Ensure render has a sensible fallback",
	}
	if err := writeStore(storePath, storeFile{Version: 1, Items: []HandoffRecord{rec}}); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out bytes.Buffer
	if err := cmdRender([]string{"--id", "latest-like"}, &out); err != nil {
		t.Fatalf("cmdRender failed: %v", err)
	}
	if !strings.Contains(out.String(), "Target tool: generic") {
		t.Fatalf("expected generic fallback target, got: %s", out.String())
	}
}

func TestCmdSaveInvalidProjectPath(t *testing.T) {
	bad := string([]byte{0x00})
	err := cmdSave([]string{"--tool", "codex", "--project", bad, "--title", "x", "--summary", "y"}, io.Discard)
	if err == nil {
		t.Fatalf("expected project path resolution error")
	}
}

func TestCmdSaveRejectsEmptyNextItem(t *testing.T) {
	tmp := t.TempDir()
	err := cmdSave([]string{
		"--tool", "codex",
		"--project", tmp,
		"--title", "Trim next",
		"--summary", "Validation",
		"--next", "   ",
	}, io.Discard)
	if err == nil {
		t.Fatalf("expected empty --next validation error")
	}
	if !strings.Contains(err.Error(), "non-empty --next") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCmdImportOnConflictSkip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	rec := HandoffRecord{
		ID:        "20260312-020000",
		CreatedAt: "2026-03-12T01:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/project",
		Title:     "Original",
		Summary:   "Original summary",
	}
	digest, err := RecordChecksum(rec)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	bundle := ExportBundle{Version: 2, Checksum: digest, Record: rec}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := os.WriteFile(bundlePath, data, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	if err := cmdImport([]string{"--input", bundlePath, "--allow-unsigned"}, io.Discard); err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	var out bytes.Buffer
	if err := cmdImport([]string{"--input", bundlePath, "--on-conflict", "skip", "--allow-unsigned"}, &out); err != nil {
		t.Fatalf("skip import failed: %v", err)
	}
	if !strings.Contains(out.String(), "skipped handoff") {
		t.Fatalf("expected skip message, got %q", out.String())
	}

	loaded, err := loadRecord(rec.ID)
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if loaded.Title != "Original" {
		t.Fatalf("record should stay unchanged after skip, got %q", loaded.Title)
	}
}

func TestCmdImportOnConflictReplace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	original := HandoffRecord{
		ID:        "20260312-020000",
		CreatedAt: "2026-03-12T01:00:00Z",
		Tool:      "codex",
		Project:   "/tmp/project",
		Title:     "Original",
		Summary:   "Original summary",
	}
	if err := updateStore(func(store *storeFile) error {
		store.Items = append(store.Items, original)
		return nil
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	replacement := original
	replacement.Title = "Updated"
	replacement.Summary = "Updated summary"
	digest, err := RecordChecksum(replacement)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	bundle := ExportBundle{Version: 2, Checksum: digest, Record: replacement}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	bundlePath := filepath.Join(tmp, "bundle-replace.json")
	if err := os.WriteFile(bundlePath, data, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	var out bytes.Buffer
	if err := cmdImport([]string{"--input", bundlePath, "--on-conflict", "replace", "--allow-unsigned"}, &out); err != nil {
		t.Fatalf("replace import failed: %v", err)
	}
	if !strings.Contains(out.String(), "replaced handoff") {
		t.Fatalf("expected replace message, got %q", out.String())
	}

	loaded, err := loadRecord(replacement.ID)
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if loaded.Title != "Updated" || loaded.Summary != "Updated summary" {
		t.Fatalf("record was not replaced: %+v", loaded)
	}
}

func TestCmdImportRejectsInvalidOnConflict(t *testing.T) {
	err := cmdImport([]string{"--input", "bundle.json", "--on-conflict", "merge"}, io.Discard)
	if err == nil {
		t.Fatalf("expected invalid on-conflict error")
	}
	if !strings.Contains(err.Error(), "invalid --on-conflict") {
		t.Fatalf("unexpected error: %v", err)
	}
}
