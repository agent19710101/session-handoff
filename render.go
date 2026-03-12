package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

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

func recordChecksum(rec handoffRecord) (string, error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("encode record checksum payload: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
