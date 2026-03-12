package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("session-handoff: use save|list|render")
		os.Exit(2)
	}
	fmt.Println("session-handoff: scaffold ready")
}
