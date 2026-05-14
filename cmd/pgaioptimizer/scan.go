package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ujjwalgupta983/pgaioptimizer/internal/analyzer"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/collector"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/engine"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/report"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a one-shot health scan on the target PostgreSQL instance",
	Run: func(cmd *cobra.Command, args []string) {
		runScan()
	},
}

func runScan() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	fmt.Println("Connecting to PostgreSQL...")
	coll, err := collector.New(ctx, dsn)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer coll.Close()

	info, err := coll.ServerInfo(ctx)
	if err != nil {
		fmt.Printf("Failed to get server info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connected to PostgreSQL %s\n", info.Version)
	fmt.Println("Running analyzers...")

	registry := analyzer.NewRegistry()
	var scores []*models.CategoryScore

	for _, a := range registry.All() {
		fmt.Printf("  -> Running %s analyzer...\n", a.Name())
		score, err := a.Analyze(ctx, coll.Pool(), info)
		if err != nil {
			fmt.Printf("Warning: analyzer %s failed: %v\n", a.Name(), err)
			continue
		}
		scores = append(scores, score)
	}

	fmt.Println("Correlating findings...")
	corr := engine.NewCorrelator()
	correlations := corr.Correlate(scores)

	fmt.Println("Scoring overall health...")
	scoreEngine := engine.NewScoringEngine()
	healthReport := scoreEngine.GenerateReport(info, scores, correlations)

	outputFile := viper.GetString("output")
	if outputFile == "" {
		outputFile = "pgaioptimizer_report.json"
	}

	fmt.Printf("Generating report to %s...\n", outputFile)
	if err := report.GenerateJSON(healthReport, outputFile); err != nil {
		fmt.Printf("Failed to generate report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n--- Scan Complete ---\n")
	fmt.Printf("Overall Grade: %s (%.1f/100)\n", healthReport.Grade, healthReport.OverallScore)
	totalFindings := 0
	for _, c := range healthReport.Categories {
		totalFindings += len(c.Findings)
	}
	fmt.Printf("Found %d issues and %d cross-category correlations.\n", totalFindings, len(healthReport.Correlations))
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().StringP("output", "o", "report.html", "Output file for the health report")
}
