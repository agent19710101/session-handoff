package main

import (
	"os"

	"github.com/agent19710101/session-handoff/internal/handoff"
)

func main() {
	if err := handoff.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
