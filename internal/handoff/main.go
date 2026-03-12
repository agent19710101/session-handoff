package handoff

import (
	"errors"
	"fmt"
	"io"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		printUsage(stdout)
		return errors.New("missing command")
	}

	var err error
	switch args[0] {
	case "save":
		err = cmdSave(args[1:], stdout)
	case "list":
		err = cmdList(args[1:], stdout)
	case "render":
		err = cmdRender(args[1:], stdout)
	case "export":
		err = cmdExport(args[1:], stdout)
	case "import":
		err = cmdImport(args[1:], stdout)
	case "select":
		err = cmdSelect(args[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return err
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "session-handoff: portable handoff bundles for AI coding sessions")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  session-handoff save --tool <name> --project <path> --title <text> --summary <text> [--next <item>]...")
	fmt.Fprintln(w, "  session-handoff list [--json] [--id <prefix>] [--tool <name>] [--project <path>] [--query <text>] [--since <duration>] [--latest] [--limit <n>]")
	fmt.Fprintln(w, "  session-handoff render --id <id|prefix|latest> [--target <tool>]")
	fmt.Fprintln(w, "  session-handoff export --id <id|prefix|latest> [--format markdown|json] [--target <tool>] [--output handoff.md]")
	fmt.Fprintln(w, "  session-handoff import --input handoff.json [--on-conflict fail|skip|replace] [--passphrase <text>] [--allow-unsigned]")
	fmt.Fprintln(w, "  session-handoff select [--query <text>] [--id <prefix>] [--tool <name>] [--project <path>] [--since <duration>] [--limit <n>] [--print-id]")
}
