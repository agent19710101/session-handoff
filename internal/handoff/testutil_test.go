package handoff

import (
	"os/exec"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, string(out))
	}
}
