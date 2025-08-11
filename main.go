// Package main provides the entry point for the remap CLI tool.
// It delegates execution to the cmd package to maintain clean separation
// between main entry logic and command implementation details.
package main

import (
	"remap/cmd"
)

func main() {
	cmd.Execute()
}
