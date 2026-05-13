package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a one-shot health scan on the target PostgreSQL instance",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running scan...")
		// TODO: Initialize collector, analyzers, engine, and output report
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringP("output", "o", "report.html", "Output file for the health report")
}
