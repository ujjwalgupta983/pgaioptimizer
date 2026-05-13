package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run as a central dashboard server to manage multiple PostgreSQL instances",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting central server...")
		// TODO: Load config, initialize API server and schedule jobs
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().String("listen", ":8080", "Address to listen on")
	serverCmd.Flags().String("instances-config", "instances.yaml", "Configuration file for managed instances")
}
