package main

import (
	"fmt"
	"os"
)

const minArgs = 2

func main() {
	if len(os.Args) < minArgs {
		fmt.Fprintln(os.Stderr, "Usage: golem <command> [arguments]")
		fmt.Fprintln(os.Stderr, "Commands: build, run")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		fmt.Println("golem build: not yet implemented")
	case "run":
		fmt.Println("golem run: not yet implemented")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
