package handoff

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func filterByTool(items []HandoffRecord, tool string) []HandoffRecord {
	wanted := strings.ToLower(strings.TrimSpace(tool))
	if wanted == "" {
		return items
	}
	filtered := make([]HandoffRecord, 0, len(items))
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Tool)) == wanted {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func parseSinceDuration(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid --since duration %q: %w", raw, err)
	}
	if d <= 0 {
		return 0, errors.New("--since must be > 0")
	}
	return d, nil
}

func filterBySince(items []HandoffRecord, now time.Time, window time.Duration) []HandoffRecord {
	cutoff := now.Add(-window)
	filtered := make([]HandoffRecord, 0, len(items))
	for _, item := range items {
		ts, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			continue
		}
		if !ts.Before(cutoff) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterByProject(items []HandoffRecord, project string) []HandoffRecord {
	if strings.TrimSpace(project) == "" {
		return items
	}
	wanted, err := filepath.Abs(strings.TrimSpace(project))
	if err != nil {
		wanted = strings.TrimSpace(project)
	}
	wanted = filepath.Clean(wanted)

	filtered := make([]HandoffRecord, 0, len(items))
	for _, item := range items {
		if filepath.Clean(item.Project) == wanted {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func pickRecord(items []HandoffRecord, id string) (HandoffRecord, error) {
	if strings.TrimSpace(id) == "latest" {
		latest := items[0]
		for _, it := range items[1:] {
			if it.CreatedAt > latest.CreatedAt {
				latest = it
			}
		}
		return latest, nil
	}
	for _, it := range items {
		if it.ID == id {
			return it, nil
		}
	}
	return HandoffRecord{}, fmt.Errorf("handoff id %q not found", id)
}

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, strings.TrimSpace(value))
	return nil
}
