package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run as a continuous monitoring agent on the PostgreSQL server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting agent...")
		// TODO: Initialize DuckDB storage, periodic scheduler, and HTTP dashboard
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().String("listen", ":9090", "Address to listen on for the web dashboard")
	agentCmd.Flags().Duration("interval", 60, "Snapshot collection interval in seconds")
	agentCmd.Flags().String("data-dir", "./pgaioptimizer-data", "Directory to store DuckDB snapshots")
}
