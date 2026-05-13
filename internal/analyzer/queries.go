package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	queriesWeight = 0.15 // 15% of overall score
)

const queryCheckStatStatements = `
SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements');
`

const querySlowQueries = `
SELECT 
    query, 
    calls, 
    total_exec_time, 
    mean_exec_time, 
    shared_blks_hit, 
    shared_blks_read,
    temp_blks_read, 
    temp_blks_written
FROM pg_stat_statements
WHERE calls > 100
ORDER BY total_exec_time DESC
LIMIT 10
`

type QueryAnalyzer struct{}

func (a *QueryAnalyzer) Name() string        { return "queries" }
func (a *QueryAnalyzer) Description() string { return "Query performance via pg_stat_statements" }
func (a *QueryAnalyzer) Weight() float64     { return queriesWeight }

func (a *QueryAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	var hasExt bool
	err := pool.QueryRow(ctx, queryCheckStatStatements).Scan(&hasExt)
	if err != nil || !hasExt {
		score.Score -= 10
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "pg_stat_statements extension is missing",
			Severity:         models.SeverityWarning,
			Description:      "pg_stat_statements is required for deep query analysis. Without it, we cannot identify slow queries, disk spills, or cache misses per query.",
			CurrentValue:     "Not installed",
			RecommendedValue: "Installed",
			Impact:           "Enables identification of the exact queries causing high CPU or disk I/O.",
			SQLFix:           "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;\n-- Note: requires shared_preload_libraries = 'pg_stat_statements' in postgresql.conf",
		})
		return score, nil
	}

	a.analyzeTopQueries(ctx, pool, score)

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *QueryAnalyzer) analyzeTopQueries(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, querySlowQueries)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var query string
		var calls int64
		var totalTime, meanTime float64
		var sharedHit, sharedRead, tempRead, tempWritten int64

		if err := rows.Scan(&query, &calls, &totalTime, &meanTime, &sharedHit, &sharedRead, &tempRead, &tempWritten); err != nil {
			continue
		}

		// Truncate query for display
		displayQuery := query
		if len(displayQuery) > 100 {
			displayQuery = displayQuery[:97] + "..."
		}
		// Clean up newlines
		displayQuery = strings.ReplaceAll(displayQuery, "\n", " ")

		// Check for disk spill
		if tempWritten > 1000 {
			score.Score -= 5
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            "Query is spilling to disk",
				Severity:         models.SeverityWarning,
				Description:      fmt.Sprintf("A query executed %d times wrote %d temporary blocks to disk. This usually means work_mem is too low for the sort or hash operation.", calls, tempWritten),
				CurrentValue:     fmt.Sprintf("%d temp blocks written", tempWritten),
				RecommendedValue: "0 temp blocks written",
				Impact:           "Writing to disk is 10-100x slower than sorting in memory.",
				SQLFix:           fmt.Sprintf("EXPLAIN (ANALYZE, BUFFERS)\n%s", displayQuery),
			})
		}
	}
}
