package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func detectChangedFiles(project string) ([]string, error) {
	cmd := exec.Command("git", "-C", project, "status", "--short")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, nil // not a git repo or other git issue: keep save usable
		}
		return nil, fmt.Errorf("collect changed files: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		files = append(files, strings.Join(parts[1:], " "))
	}
	return files, nil
}
