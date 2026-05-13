package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	tablesWeight = 0.10 // 10% of overall score
)

const queryTableStats = `
SELECT 
    schemaname, 
    relname, 
    seq_scan, 
    seq_tup_read, 
    idx_scan, 
    idx_tup_fetch, 
    n_live_tup, 
    n_dead_tup,
    pg_relation_size(relid) AS table_bytes
FROM pg_stat_user_tables
WHERE n_live_tup > 10000  -- only analyze reasonably sized tables
ORDER BY seq_scan DESC
`

type TableAnalyzer struct{}

func (a *TableAnalyzer) Name() string        { return "tables" }
func (a *TableAnalyzer) Description() string { return "Table health: sequential scans and table size" }
func (a *TableAnalyzer) Weight() float64     { return tablesWeight }

func (a *TableAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeSeqScans(ctx, pool, score)

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *TableAnalyzer) analyzeSeqScans(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryTableStats)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		var seqScan, seqTupRead, idxScan, idxTupFetch, liveTup, deadTup, tableBytes int64
		if err := rows.Scan(&schema, &table, &seqScan, &seqTupRead, &idxScan, &idxTupFetch, &liveTup, &deadTup, &tableBytes); err != nil {
			continue
		}

		tableMB := tableBytes / (1024 * 1024)

		// Heuristic: If table is >10MB, has >1000 seq scans, and seq scans are 10x more frequent than index scans
		if tableMB > 10 && seqScan > 1000 && seqScan > (idxScan*10) {
			score.Score -= 5
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("High sequential scans on %s.%s", schema, table),
				Severity:         models.SeverityWarning,
				Description:      fmt.Sprintf("Table %s.%s (%d MB, %d rows) has %d sequential scans compared to only %d index scans.", schema, table, tableMB, liveTup, seqScan, idxScan),
				CurrentValue:     fmt.Sprintf("%d seq scans", seqScan),
				RecommendedValue: "Use index scans for large tables",
				Impact:           "Full table scans are very slow and evict useful data from the cache.",
				SQLFix:           fmt.Sprintf("-- Investigate queries hitting %s.%s and add appropriate indexes", schema, table),
			})
		}
	}
}
