package handoff

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePorcelainV1ZIncludesSpacesUnicodeAndRename(t *testing.T) {
	out := []byte(
		" M dir/normal.go\x00" +
			"?? docs/spec with spaces.md\x00" +
			"R  new name.txt\x00old name.txt\x00" +
			"A  unicodé/żółw.txt\x00",
	)

	got, err := parsePorcelainV1Z(out)
	if err != nil {
		t.Fatalf("parsePorcelainV1Z failed: %v", err)
	}

	want := []string{
		"dir/normal.go",
		"docs/spec with spaces.md",
		"old name.txt -> new name.txt",
		"unicodé/żółw.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected parsed paths\nwant: %#v\n got: %#v", want, got)
	}
}

func TestParsePorcelainV1ZRejectsMalformedRecord(t *testing.T) {
	_, err := parsePorcelainV1Z([]byte("bad-record\x00"))
	if err == nil {
		t.Fatalf("expected malformed record error")
	}
}

func TestParsePorcelainV1ZRejectsRenameMissingSource(t *testing.T) {
	_, err := parsePorcelainV1Z([]byte("R  only-destination.txt\x00"))
	if err == nil {
		t.Fatalf("expected rename missing source error")
	}
}

func TestDetectChangedFilesReturnsGitFailure(t *testing.T) {
	_, err := detectChangedFiles(t.TempDir())
	if err == nil {
		t.Fatalf("expected git status error for non-repo path")
	}
	if !strings.Contains(err.Error(), "git status failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
