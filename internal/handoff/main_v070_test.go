package handoff

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportImportSignedEncryptedBundle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := cmdSave([]string{"--tool", "codex", "--project", tmp, "--title", "Signed", "--summary", "Encrypted transfer"}, io.Discard); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(tmp, "signer.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	bundlePath := filepath.Join(tmp, "bundle.enc.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath, "--sign-key", keyPath, "--signer", "agent", "--passphrase", "secret"}, io.Discard); err != nil {
		t.Fatalf("export: %v", err)
	}

	if err := os.Remove(filepath.Join(tmp, "session-handoff", "handoffs.json")); err != nil {
		t.Fatalf("remove store: %v", err)
	}

	if err := cmdImport([]string{"--input", bundlePath, "--passphrase", "secret"}, io.Discard); err != nil {
		t.Fatalf("import signed encrypted: %v", err)
	}
}

func TestImportRejectsUnsignedByDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := cmdSave([]string{"--tool", "codex", "--project", tmp, "--title", "Unsigned", "--summary", "Legacy"}, io.Discard); err != nil {
		t.Fatalf("save: %v", err)
	}
	bundlePath := filepath.Join(tmp, "bundle.json")
	if err := cmdExport([]string{"--id", "latest", "--format", "json", "--output", bundlePath}, io.Discard); err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := os.Remove(filepath.Join(tmp, "session-handoff", "handoffs.json")); err != nil {
		t.Fatalf("remove store: %v", err)
	}
	err := cmdImport([]string{"--input", bundlePath}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "allow-unsigned") {
		t.Fatalf("expected unsigned rejection, got %v", err)
	}
}

func TestSelectPrintID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	if err := cmdSave([]string{"--tool", "codex", "--project", tmp, "--title", "one", "--summary", "s"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := cmdSave([]string{"--tool", "codex", "--project", tmp, "--title", "two", "--summary", "s"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := cmdSelect([]string{"--query", "two", "--print-id"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatalf("expected id output")
	}
}

func TestSelectSupportsIDPrefixAndSince(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	now := time.Now().UTC()
	store := storeFile{
		Version: 1,
		Items: []HandoffRecord{
			{ID: "20260312-010000", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), Tool: "codex", Project: tmp, Title: "old", Summary: "s"},
			{ID: "20260312-020000", CreatedAt: now.Add(-20 * time.Minute).Format(time.RFC3339), Tool: "codex", Project: tmp, Title: "recent", Summary: "s"},
			{ID: "20260311-230000", CreatedAt: now.Add(-10 * time.Minute).Format(time.RFC3339), Tool: "codex", Project: tmp, Title: "other-day", Summary: "s"},
		},
	}
	storePath := filepath.Join(tmp, "session-handoff", "handoffs.json")
	if err := writeStore(storePath, store); err != nil {
		t.Fatalf("write store: %v", err)
	}

	var out strings.Builder
	if err := cmdSelect([]string{"--id", "20260312", "--since", "30m", "--print-id"}, &out); err != nil {
		t.Fatalf("cmdSelect failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "20260312-020000" {
		t.Fatalf("expected filtered id 20260312-020000, got %q", got)
	}
}

func TestSelectRejectsInvalidFlags(t *testing.T) {
	if err := cmdSelect([]string{"--limit", "-1"}, io.Discard); err == nil {
		t.Fatalf("expected validation error for negative --limit")
	}
	if err := cmdSelect([]string{"--since", "later"}, io.Discard); err == nil {
		t.Fatalf("expected validation error for invalid --since")
	}
}
