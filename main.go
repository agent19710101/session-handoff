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

	recID := generateUniqueID(store.Items, now)
	if recID == "" {
		return errors.New("could not allocate unique handoff id")
	}

	rec := handoffRecord{
		ID:        recID,
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
