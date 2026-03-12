package handoff

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agent19710101/session-handoff/pkg/handoffid"
)

func cmdSave(args []string, stdout io.Writer) error {
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

	normalizedNext, err := normalizeNextItems(next)
	if err != nil {
		return err
	}

	absProject, err := filepath.Abs(*project)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}

	now := time.Now().UTC()
	changed, err := detectChangedFiles(absProject)
	if err != nil {
		return err
	}

	rec := HandoffRecord{
		CreatedAt: now.Format(time.RFC3339),
		Tool:      strings.TrimSpace(*tool),
		Project:   absProject,
		Title:     strings.TrimSpace(*title),
		Summary:   strings.TrimSpace(*summary),
		Next:      normalizedNext,
		Changed:   changed,
	}

	if err := updateStore(func(store *storeFile) error {
		existing := make([]string, 0, len(store.Items))
		for _, it := range store.Items {
			existing = append(existing, it.ID)
		}
		recID := handoffid.Generate(existing, now)
		if recID == "" {
			return errors.New("could not allocate unique handoff id")
		}
		rec.ID = recID
		if err := validateRecord(rec); err != nil {
			return err
		}
		store.Items = append(store.Items, rec)
		return nil
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "saved handoff %s\n", rec.ID)
	return nil
}

func normalizeNextItems(next []string) ([]string, error) {
	cleaned := make([]string, 0, len(next))
	for _, item := range next {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil, errors.New("save requires non-empty --next items")
		}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned, nil
}

func cmdList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print records as json")
	idPrefix := fs.String("id", "", "filter by handoff id prefix")
	tool := fs.String("tool", "", "filter by source tool")
	project := fs.String("project", "", "filter by project path")
	query := fs.String("query", "", "case-insensitive substring filter across title/summary/next")
	limit := fs.Int("limit", 0, "max number of most-recent records to show (0 = all)")
	latest := fs.Bool("latest", false, "show only the most recent record")
	since := fs.String("since", "", "show records from the last duration (e.g. 30m, 6h, 7h30m)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 0 {
		return errors.New("--limit must be >= 0")
	}
	if *latest && *limit > 0 {
		return errors.New("--latest cannot be combined with --limit")
	}

	sinceDuration, err := parseSinceDuration(*since)
	if err != nil {
		return err
	}

	store, _, err := loadStore()
	if err != nil {
		return err
	}

	items := append([]HandoffRecord(nil), store.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})

	if strings.TrimSpace(*idPrefix) != "" {
		items = filterByIDPrefix(items, *idPrefix)
	}
	if strings.TrimSpace(*tool) != "" {
		items = filterByTool(items, *tool)
	}
	if strings.TrimSpace(*project) != "" {
		items = filterByProject(items, *project)
	}
	if strings.TrimSpace(*query) != "" {
		items = filterByQuery(items, *query)
	}
	if sinceDuration > 0 {
		items = filterBySince(items, time.Now().UTC(), sinceDuration)
	}
	if *latest && len(items) > 1 {
		items = items[:1]
	}
	if *limit > 0 && len(items) > *limit {
		items = items[:*limit]
	}

	if len(items) == 0 {
		if *asJSON {
			fmt.Fprintln(stdout, "[]")
			return nil
		}
		fmt.Fprintln(stdout, "no handoffs saved")
		return nil
	}

	if *asJSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("encode json list: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	for _, item := range items {
		fmt.Fprintf(stdout, "%s  %-12s  %s\n", item.ID, item.Tool, item.Title)
		fmt.Fprintf(stdout, "  project: %s\n", item.Project)
		if len(item.Next) > 0 {
			fmt.Fprintf(stdout, "  next: %s\n", strings.Join(item.Next, " | "))
		}
	}
	return nil
}

func cmdRender(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	id := fs.String("id", "latest", "handoff id, unique id prefix, or latest")
	target := fs.String("target", "generic", "target tool")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rec, err := loadRecord(*id)
	if err != nil {
		return err
	}

	selectedTarget := strings.TrimSpace(*target)
	if selectedTarget == "" {
		selectedTarget = "generic"
	}

	fmt.Fprint(stdout, RenderMarkdown(rec, selectedTarget))
	return nil
}

func cmdExport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	id := fs.String("id", "latest", "handoff id, unique id prefix, or latest")
	out := fs.String("output", "", "file path (default stdout)")
	format := fs.String("format", "markdown", "output format: markdown|json")
	target := fs.String("target", "generic", "target tool used for markdown export context")
	signKey := fs.String("sign-key", "", "ed25519 PKCS8 private key PEM for signing JSON bundle")
	signer := fs.String("signer", "", "signer display name metadata")
	passphrase := fs.String("passphrase", "", "encrypt/decrypt passphrase for JSON bundles")
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
		selectedTarget := strings.TrimSpace(*target)
		if selectedTarget == "" {
			selectedTarget = "generic"
		}
		payload = RenderMarkdown(rec, selectedTarget)
	case "json":
		digest, err := RecordChecksum(rec)
		if err != nil {
			return err
		}
		bundle := ExportBundle{Version: 2, Checksum: digest, Record: rec}
		if strings.TrimSpace(*signKey) != "" {
			priv, pub, err := loadPrivateKey(strings.TrimSpace(*signKey))
			if err != nil {
				return err
			}
			bundle.Version = 3
			bundle.Signer = &SignerMeta{
				Name:      strings.TrimSpace(*signer),
				KeyID:     publicKeyID(pub),
				PublicKey: base64.StdEncoding.EncodeToString(pub),
			}
			bundle.Signature = signChecksum(priv, digest)
		}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return fmt.Errorf("encode export bundle: %w", err)
		}
		if strings.TrimSpace(*passphrase) != "" {
			enc, err := encryptBundle(data, *passphrase)
			if err != nil {
				return err
			}
			encData, err := json.MarshalIndent(enc, "", "  ")
			if err != nil {
				return fmt.Errorf("encode encrypted export bundle: %w", err)
			}
			payload = string(encData) + "\n"
		} else {
			payload = string(data) + "\n"
		}
	default:
		return fmt.Errorf("unsupported export format %q", *format)
	}

	if strings.TrimSpace(*out) == "" {
		fmt.Fprint(stdout, payload)
		return nil
	}
	if err := os.WriteFile(*out, []byte(payload), 0o644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}
	fmt.Fprintf(stdout, "exported %s\n", *out)
	return nil
}

func cmdImport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	input := fs.String("input", "", "json bundle file path")
	onConflict := fs.String("on-conflict", "fail", "how to handle existing handoff id: fail|skip|replace")
	passphrase := fs.String("passphrase", "", "decrypt passphrase for encrypted bundles")
	allowUnsigned := fs.Bool("allow-unsigned", false, "allow importing unsigned v2 bundles")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return errors.New("import requires --input")
	}

	conflictMode := strings.ToLower(strings.TrimSpace(*onConflict))
	switch conflictMode {
	case "", "fail", "skip", "replace":
		if conflictMode == "" {
			conflictMode = "fail"
		}
	default:
		return fmt.Errorf("invalid --on-conflict value %q (expected fail|skip|replace)", *onConflict)
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	var encProbe EncryptedBundle
	if err := json.Unmarshal(data, &encProbe); err == nil && strings.TrimSpace(encProbe.Ciphertext) != "" {
		if strings.TrimSpace(*passphrase) == "" {
			return errors.New("encrypted bundle requires --passphrase")
		}
		plain, err := decryptBundle(encProbe, *passphrase)
		if err != nil {
			return err
		}
		data = plain
	}

	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("parse import bundle: %w", err)
	}
	if bundle.Version == 0 {
		return errors.New("import bundle missing version")
	}
	rec := bundle.Record
	if strings.TrimSpace(bundle.Checksum) != "" {
		digest, err := RecordChecksum(rec)
		if err != nil {
			return err
		}
		if digest != bundle.Checksum {
			return errors.New("import bundle checksum mismatch")
		}
	} else if bundle.Version >= 2 {
		return errors.New("import bundle missing checksum")
	}
	if strings.TrimSpace(bundle.Signature) != "" {
		if err := verifyBundleSignature(bundle); err != nil {
			return err
		}
	} else if bundle.Version >= 3 {
		return errors.New("import bundle missing signature metadata")
	} else if bundle.Version == 2 && !*allowUnsigned {
		return errors.New("unsigned v2 bundle blocked; re-run with --allow-unsigned to import legacy checksum-only bundles")
	}
	if strings.TrimSpace(rec.ID) == "" || strings.TrimSpace(rec.Tool) == "" || strings.TrimSpace(rec.Project) == "" || strings.TrimSpace(rec.Title) == "" || strings.TrimSpace(rec.Summary) == "" {
		return errors.New("import bundle missing required record fields")
	}
	if err := validateRecord(rec); err != nil {
		return fmt.Errorf("import bundle invalid record: %w", err)
	}

	var result string
	if err := updateStore(func(store *storeFile) error {
		for i, existing := range store.Items {
			if existing.ID != rec.ID {
				continue
			}
			switch conflictMode {
			case "skip":
				result = "skipped"
				return nil
			case "replace":
				store.Items[i] = rec
				result = "replaced"
				return nil
			default:
				return fmt.Errorf("handoff %s already exists", rec.ID)
			}
		}
		store.Items = append(store.Items, rec)
		result = "imported"
		return nil
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s handoff %s\n", result, rec.ID)
	return nil
}

func cmdSelect(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("select", flag.ContinueOnError)
	query := fs.String("query", "", "case-insensitive filter by title/summary/next")
	idPrefix := fs.String("id", "", "filter by handoff id prefix")
	tool := fs.String("tool", "", "filter by source tool")
	project := fs.String("project", "", "filter by project path")
	since := fs.String("since", "", "show records from the last duration (e.g. 30m, 6h, 7h30m)")
	limit := fs.Int("limit", 0, "max number of records to show")
	printID := fs.Bool("print-id", false, "print only selected id")
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
	items := append([]HandoffRecord(nil), store.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	if strings.TrimSpace(*idPrefix) != "" {
		items = filterByIDPrefix(items, *idPrefix)
	}
	if strings.TrimSpace(*tool) != "" {
		items = filterByTool(items, *tool)
	}
	if strings.TrimSpace(*project) != "" {
		items = filterByProject(items, *project)
	}
	if strings.TrimSpace(*query) != "" {
		items = filterByQuery(items, *query)
	}
	if sinceDuration > 0 {
		items = filterBySince(items, time.Now().UTC(), sinceDuration)
	}
	if *limit > 0 && len(items) > *limit {
		items = items[:*limit]
	}
	if len(items) == 0 {
		return errors.New("no handoffs match selector filters")
	}
	if *printID {
		fmt.Fprintln(stdout, items[0].ID)
		return nil
	}

	for i, item := range items {
		fmt.Fprintf(stdout, "%d) %s  %-10s %s\n", i+1, item.ID, item.Tool, item.Title)
	}
	fmt.Fprint(stdout, "Select number: ")
	var input string
	if _, err := fmt.Fscanln(os.Stdin, &input); err != nil {
		return fmt.Errorf("read selection: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || n < 1 || n > len(items) {
		return errors.New("invalid selection")
	}
	fmt.Fprintln(stdout, items[n-1].ID)
	return nil
}
