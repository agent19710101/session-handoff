package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type handoffRecord struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"created_at"`
	Tool      string   `json:"tool"`
	Project   string   `json:"project"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Next      []string `json:"next"`
}

type storeFile struct {
	Version int             `json:"version"`
	Items   []handoffRecord `json:"items"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "save":
		err = cmdSave(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "render":
		err = cmdRender(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("session-handoff: portable handoff bundles for AI coding sessions")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  session-handoff save --tool <name> --project <path> --title <text> --summary <text> [--next <item>]...")
	fmt.Println("  session-handoff list")
	fmt.Println("  session-handoff render --id <id|latest> --target <tool>")
}

func cmdSave(args []string) error {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	tool := fs.String("tool", "", "source tool name (codex, claude-code, cursor, etc)")
	project := fs.String("project", "", "project path")
	title := fs.String("title", "", "short title")
	summary := fs.String("summary", "", "what was done/current state")
	var next multiFlag
	fs.Var(&next, "next", "next step (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*tool) == "" || strings.TrimSpace(*project) == "" || strings.TrimSpace(*title) == "" || strings.TrimSpace(*summary) == "" {
		return errors.New("save requires --tool, --project, --title, --summary")
	}

	absProject, err := filepath.Abs(*project)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}

	store, path, err := loadStore()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	rec := handoffRecord{
		ID:        now.Format("20060102-150405"),
		CreatedAt: now.Format(time.RFC3339),
		Tool:      strings.TrimSpace(*tool),
		Project:   absProject,
		Title:     strings.TrimSpace(*title),
		Summary:   strings.TrimSpace(*summary),
		Next:      next,
	}

	store.Items = append(store.Items, rec)
	if err := writeStore(path, store); err != nil {
		return err
	}

	fmt.Printf("saved handoff %s\n", rec.ID)
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store, _, err := loadStore()
	if err != nil {
		return err
	}
	if len(store.Items) == 0 {
		fmt.Println("no handoffs saved")
		return nil
	}

	items := append([]handoffRecord(nil), store.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})

	for _, item := range items {
		fmt.Printf("%s  %-12s  %s\n", item.ID, item.Tool, item.Title)
		fmt.Printf("  project: %s\n", item.Project)
		if len(item.Next) > 0 {
			fmt.Printf("  next: %s\n", strings.Join(item.Next, " | "))
		}
	}
	return nil
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	id := fs.String("id", "latest", "handoff id or latest")
	target := fs.String("target", "", "target tool")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*target) == "" {
		return errors.New("render requires --target")
	}

	store, _, err := loadStore()
	if err != nil {
		return err
	}
	if len(store.Items) == 0 {
		return errors.New("no handoffs to render")
	}

	rec, err := pickRecord(store.Items, *id)
	if err != nil {
		return err
	}

	fmt.Println("# Session Handoff")
	fmt.Printf("- Source tool: %s\n", rec.Tool)
	fmt.Printf("- Target tool: %s\n", strings.TrimSpace(*target))
	fmt.Printf("- Project: %s\n", rec.Project)
	fmt.Printf("- Created: %s\n", rec.CreatedAt)
	fmt.Printf("- Topic: %s\n", rec.Title)
	fmt.Println()
	fmt.Println("## Current state")
	fmt.Println(rec.Summary)
	fmt.Println()
	fmt.Println("## Requested continuation")
	if len(rec.Next) == 0 {
		fmt.Println("- Continue from current state and produce a small, verifiable next change.")
	} else {
		for _, step := range rec.Next {
			fmt.Printf("- %s\n", step)
		}
	}
	fmt.Println()
	fmt.Println("## Constraints")
	fmt.Println("- Keep commits small and reviewable.")
	fmt.Println("- Run formatting and tests before finalizing.")
	fmt.Println("- Summarize changes, risks, and follow-up tasks.")

	return nil
}

func pickRecord(items []handoffRecord, id string) (handoffRecord, error) {
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
	return handoffRecord{}, fmt.Errorf("handoff id %q not found", id)
}

func loadStore() (storeFile, string, error) {
	path, err := defaultStorePath()
	if err != nil {
		return storeFile{}, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storeFile{Version: 1, Items: []handoffRecord{}}, path, nil
		}
		return storeFile{}, "", fmt.Errorf("read store: %w", err)
	}

	var s storeFile
	if err := json.Unmarshal(data, &s); err != nil {
		return storeFile{}, "", fmt.Errorf("parse store: %w", err)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Items == nil {
		s.Items = []handoffRecord{}
	}
	return s, path, nil
}

func writeStore(path string, s storeFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func defaultStorePath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(cfg, "session-handoff", "handoffs.json"), nil
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, strings.TrimSpace(value))
	return nil
}
