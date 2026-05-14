package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/api"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run as a central dashboard server to manage multiple PostgreSQL instances",
	Run: func(cmd *cobra.Command, args []string) {
		listenAddr := viper.GetString("listen")
		if listenAddr == "" {
			listenAddr = ":8080"
		}

		fmt.Printf("Starting central server on %s...\n", listenAddr)

		srv := api.NewServer(listenAddr)

		if err := srv.Start(); err != nil {
			fmt.Printf("Server failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().String("listen", ":8080", "Address to listen on")
	serverCmd.Flags().String("instances-config", "instances.yaml", "Configuration file for managed instances")
}
