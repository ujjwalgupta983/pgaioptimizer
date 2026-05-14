package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ujjwalgupta983/pgaioptimizer/internal/analyzer"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/api"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/collector"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/engine"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/storage"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run as a continuous monitoring agent on the PostgreSQL server",
	Run: func(cmd *cobra.Command, args []string) {
		runAgent(cmd)
	},
}

func runAgent(cmd *cobra.Command) {
	fmt.Println("Starting pgaioptimizer agent...")

	// Initialize DuckDB Store
	dataDir, _ := cmd.Flags().GetString("data-dir")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("Failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dataDir, "pgaioptimizer.duckdb")
	store, err := storage.NewDuckDBStore(dbPath)
	if err != nil {
		fmt.Printf("Failed to initialize DuckDB storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Start API Server
	listenAddr, _ := cmd.Flags().GetString("listen")
	srv := api.NewServer(listenAddr)
	go func() {
		if err := srv.Start(); err != nil {
			fmt.Printf("API Server failed: %v\n", err)
		}
	}()

	intervalSecs, _ := cmd.Flags().GetInt("interval")
	interval := time.Duration(intervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	dsn := viper.GetString("dsn")
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			viper.GetString("user"),
			viper.GetString("password"),
			viper.GetString("host"),
			viper.GetInt("port"),
			viper.GetString("db"),
		)
	}

	fmt.Printf("Agent started. Collecting snapshots every %v...\n", interval)

	// Run first snapshot immediately
	runSnapshot(dsn, store, srv)

	for {
		select {
		case <-ticker.C:
			runSnapshot(dsn, store, srv)
		case sig := <-sigChan:
			fmt.Printf("Received signal %v. Shutting down...\n", sig)
			return
		}
	}
}

func runSnapshot(dsn string, store *storage.DuckDBStore, srv *api.Server) {
	fmt.Printf("[%s] Collecting snapshot...\n", time.Now().Format(time.RFC3339))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	coll, err := collector.New(ctx, dsn)
	if err != nil {
		fmt.Printf("Snapshot failed (connection): %v\n", err)
		return
	}
	defer coll.Close()

	info, err := coll.ServerInfo(ctx)
	if err != nil {
		fmt.Printf("Snapshot failed (info): %v\n", err)
		return
	}

	registry := analyzer.NewRegistry()
	var scores []*models.CategoryScore

	for _, a := range registry.All() {
		score, err := a.Analyze(ctx, coll.Pool(), info)
		if err != nil {
			continue // Skip failed analyzers silently in agent mode
		}
		scores = append(scores, score)
	}

	corr := engine.NewCorrelator()
	correlations := corr.Correlate(scores)

	scoreEngine := engine.NewScoringEngine()
	report := scoreEngine.GenerateReport(info, scores, correlations)
	report.GeneratedAt = time.Now()
	report.AnalysisTier = "agent"

	// Update API server's latest report
	srv.UpdateLatestReport(report)

	// Save to DuckDB
	if err := store.SaveSnapshot(report); err != nil {
		fmt.Printf("Failed to save snapshot to DuckDB: %v\n", err)
	} else {
		fmt.Printf("[%s] Snapshot saved. Grade: %s, Score: %.1f\n", time.Now().Format(time.RFC3339), report.Grade, report.OverallScore)
	}
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().String("listen", ":9090", "Address to listen on for the web dashboard")
	agentCmd.Flags().Duration("interval", 60, "Snapshot collection interval in seconds")
	agentCmd.Flags().String("data-dir", "./pgaioptimizer-data", "Directory to store DuckDB snapshots")
}
