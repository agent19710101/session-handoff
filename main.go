package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	Changed   []string `json:"changed,omitempty"`
}

type storeFile struct {
	Version int             `json:"version"`
	Items   []handoffRecord `json:"items"`
}

type exportBundle struct {
	Version  int           `json:"version"`
	Checksum string        `json:"checksum,omitempty"`
	Record   handoffRecord `json:"record"`
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
	case "export":
		err = cmdExport(os.Args[2:])
	case "import":
		err = cmdImport(os.Args[2:])
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
	fmt.Println("  session-handoff list [--json] [--tool <name>] [--project <path>] [--since <duration>] [--limit <n>]")
	fmt.Println("  session-handoff render --id <id|latest> --target <tool>")
	fmt.Println("  session-handoff export --id <id|latest> [--format markdown|json] [--output handoff.md]")
	fmt.Println("  session-handoff import --input handoff.json")
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
	changed, err := detectChangedFiles(absProject)
	if err != nil {
		return err
	}

	rec := handoffRecord{
		ID:        now.Format("20060102-150405"),
		CreatedAt: now.Format(time.RFC3339),
		Tool:      strings.TrimSpace(*tool),
		Project:   absProject,
		Title:     strings.TrimSpace(*title),
		Summary:   strings.TrimSpace(*summary),
		Next:      next,
		Changed:   changed,
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
	asJSON := fs.Bool("json", false, "print records as json")
	tool := fs.String("tool", "", "filter by source tool")
	project := fs.String("project", "", "filter by project path")
	limit := fs.Int("limit", 0, "max number of most-recent records to show (0 = all)")
	since := fs.String("since", "", "show records from the last duration (e.g. 30m, 6h, 7h30m)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 0 {
		return errors.New("--limit must be >= 0")
	}

	sinceDuration, err := parseSinceDuration(*since)
	if err != nil {
		return err
	}

	store, _, err := loadStore()
	if err != nil {
		return err
	}

	items := append([]handoffRecord(nil), store.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})

	if strings.TrimSpace(*tool) != "" {
		items = filterByTool(items, *tool)
	}
	if strings.TrimSpace(*project) != "" {
		items = filterByProject(items, *project)
	}
	if sinceDuration > 0 {
		items = filterBySince(items, time.Now().UTC(), sinceDuration)
	}
	if *limit > 0 && len(items) > *limit {
		items = items[:*limit]
	}

	if len(items) == 0 {
		if *asJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("no handoffs saved")
		return nil
	}

	if *asJSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("encode json list: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

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

	rec, err := loadRecord(*id)
	if err != nil {
		return err
	}

	fmt.Print(renderMarkdown(rec, strings.TrimSpace(*target)))
	return nil
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	id := fs.String("id", "latest", "handoff id or latest")
	out := fs.String("output", "", "file path (default stdout)")
	format := fs.String("format", "markdown", "output format: markdown|json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rec, err := loadRecord(*id)
	if err != nil {
		return err
	}

	var payload string
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "markdown", "md":
		payload = renderMarkdown(rec, "generic")
	case "json":
		digest, err := recordChecksum(rec)
		if err != nil {
			return err
		}
		bundle := exportBundle{Version: 2, Checksum: digest, Record: rec}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return fmt.Errorf("encode export bundle: %w", err)
		}
		payload = string(data) + "\n"
	default:
		return fmt.Errorf("unsupported export format %q", *format)
	}

	if strings.TrimSpace(*out) == "" {
		fmt.Print(payload)
		return nil
	}
	if err := os.WriteFile(*out, []byte(payload), 0o644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}
	fmt.Printf("exported %s\n", *out)
	return nil
}

func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	input := fs.String("input", "", "json bundle file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("import requires --input")
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	var bundle exportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("parse import bundle: %w", err)
	}
	if bundle.Version == 0 {
		return errors.New("import bundle missing version")
	}
	rec := bundle.Record
	if strings.TrimSpace(bundle.Checksum) != "" {
		digest, err := recordChecksum(rec)
		if err != nil {
			return err
		}
		if digest != bundle.Checksum {
			return errors.New("import bundle checksum mismatch")
		}
	} else if bundle.Version >= 2 {
		return errors.New("import bundle missing checksum")
	}
	if strings.TrimSpace(rec.ID) == "" || strings.TrimSpace(rec.CreatedAt) == "" || strings.TrimSpace(rec.Tool) == "" || strings.TrimSpace(rec.Project) == "" || strings.TrimSpace(rec.Title) == "" || strings.TrimSpace(rec.Summary) == "" {
		return errors.New("import bundle missing required record fields")
	}

	store, path, err := loadStore()
	if err != nil {
		return err
	}
	for _, existing := range store.Items {
		if existing.ID == rec.ID {
			return fmt.Errorf("handoff %s already exists", rec.ID)
		}
	}

	store.Items = append(store.Items, rec)
	if err := writeStore(path, store); err != nil {
		return err
	}

	fmt.Printf("imported handoff %s\n", rec.ID)
	return nil
}

func loadRecord(id string) (handoffRecord, error) {
	store, _, err := loadStore()
	if err != nil {
		return handoffRecord{}, err
	}
	if len(store.Items) == 0 {
		return handoffRecord{}, errors.New("no handoffs saved")
	}
	return pickRecord(store.Items, id)
}

func renderMarkdown(rec handoffRecord, target string) string {
	var b strings.Builder
	b.WriteString("# Session Handoff\n")
	b.WriteString(fmt.Sprintf("- Source tool: %s\n", rec.Tool))
	b.WriteString(fmt.Sprintf("- Target tool: %s\n", target))
	b.WriteString(fmt.Sprintf("- Project: %s\n", rec.Project))
	b.WriteString(fmt.Sprintf("- Created: %s\n", rec.CreatedAt))
	b.WriteString(fmt.Sprintf("- Topic: %s\n\n", rec.Title))
	b.WriteString("## Current state\n")
	b.WriteString(rec.Summary + "\n\n")
	if len(rec.Changed) > 0 {
		b.WriteString("## Working tree signals\n")
		for _, f := range rec.Changed {
			b.WriteString("- " + f + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Requested continuation\n")
	if len(rec.Next) == 0 {
		b.WriteString("- Continue from current state and produce a small, verifiable next change.\n\n")
	} else {
		for _, step := range rec.Next {
			b.WriteString("- " + step + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Constraints\n")
	b.WriteString("- Keep commits small and reviewable.\n")
	b.WriteString("- Run formatting and tests before finalizing.\n")
	b.WriteString("- Summarize changes, risks, and follow-up tasks.\n")
	return b.String()
}

func filterByTool(items []handoffRecord, tool string) []handoffRecord {
	wanted := strings.ToLower(strings.TrimSpace(tool))
	if wanted == "" {
		return items
	}
	filtered := make([]handoffRecord, 0, len(items))
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

func filterBySince(items []handoffRecord, now time.Time, window time.Duration) []handoffRecord {
	cutoff := now.Add(-window)
	filtered := make([]handoffRecord, 0, len(items))
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

func filterByProject(items []handoffRecord, project string) []handoffRecord {
	if strings.TrimSpace(project) == "" {
		return items
	}
	wanted, err := filepath.Abs(strings.TrimSpace(project))
	if err != nil {
		wanted = strings.TrimSpace(project)
	}
	wanted = filepath.Clean(wanted)

	filtered := make([]handoffRecord, 0, len(items))
	for _, item := range items {
		if filepath.Clean(item.Project) == wanted {
			filtered = append(filtered, item)
		}
	}
	return filtered
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

func recordChecksum(rec handoffRecord) (string, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("encode record checksum payload: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

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
