// Package main is the entrypoint for pgaioptimizer CLI.
package main

import (
	"fmt"
	"os"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: Initialize cobra root command with scan/agent/server subcommands
	fmt.Printf("pgaioptimizer %s (built %s)\n", version, buildTime)
	fmt.Println("PostgreSQL AI Performance Analyzer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  pgaioptimizer scan    Run a one-shot health scan")
	fmt.Println("  pgaioptimizer agent   Run as a continuous monitoring agent")
	fmt.Println("  pgaioptimizer server  Run as a central dashboard server")
	return nil
}
