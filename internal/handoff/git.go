package handoff

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
)

func detectChangedFiles(project string) ([]string, error) {
	cmd := exec.Command("git", "-C", project, "status", "--porcelain=v1", "-z")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, nil // not a git repo or other git issue: keep save usable
		}
		return nil, fmt.Errorf("collect changed files: %w", err)
	}
	files, err := parsePorcelainV1Z(out)
	if err != nil {
		return nil, fmt.Errorf("parse changed files: %w", err)
	}
	return files, nil
}

func parsePorcelainV1Z(out []byte) ([]string, error) {
	tokens := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(tokens))

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if len(tok) == 0 {
			continue
		}
		if len(tok) < 4 || tok[2] != ' ' {
			return nil, fmt.Errorf("invalid porcelain record %q", string(tok))
		}

		statusX := tok[0]
		statusY := tok[1]
		path := string(tok[3:])
		if path == "" {
			continue
		}

		if statusX == 'R' || statusX == 'C' || statusY == 'R' || statusY == 'C' {
			if i+1 >= len(tokens) || len(tokens[i+1]) == 0 {
				return nil, fmt.Errorf("missing source path for rename/copy %q", string(tok))
			}
			from := string(tokens[i+1])
			files = append(files, fmt.Sprintf("%s -> %s", from, path))
			i++
			continue
		}

		files = append(files, path)
	}

	sort.Strings(files)
	return files, nil
}
