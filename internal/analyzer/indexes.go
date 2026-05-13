package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	indexesWeight = 0.15 // 15% of overall score
)

const queryUnusedIndexes = `
SELECT 
    i.schemaname, 
    i.relname, 
    i.indexrelname, 
    pg_relation_size(i.indexrelid) AS index_bytes
FROM pg_stat_user_indexes i
JOIN pg_index x ON i.indexrelid = x.indexrelid
WHERE i.idx_scan = 0
  AND x.indisunique IS FALSE
  AND x.indisprimary IS FALSE
ORDER BY index_bytes DESC
LIMIT 50
`

type IndexAnalyzer struct{}

func (a *IndexAnalyzer) Name() string { return "indexes" }
func (a *IndexAnalyzer) Description() string {
	return "Index health: unused, missing, and duplicate indexes"
}
func (a *IndexAnalyzer) Weight() float64 { return indexesWeight }

func (a *IndexAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeUnusedIndexes(ctx, pool, score)

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *IndexAnalyzer) analyzeUnusedIndexes(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryUnusedIndexes)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table, index string
		var sizeBytes int64
		if err := rows.Scan(&schema, &table, &index, &sizeBytes); err != nil {
			continue
		}

		sizeMB := sizeBytes / (1024 * 1024)
		if sizeMB == 0 {
			continue // ignore very small unused indexes for now
		}

		score.Score -= 2 // small penalty per unused index
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            fmt.Sprintf("Unused index: %s", index),
			Severity:         models.SeverityInfo,
			Description:      fmt.Sprintf("Index %s on table %s.%s has never been used for a scan.", index, schema, table),
			CurrentValue:     "0 scans",
			RecommendedValue: "Drop if confirmed unused",
			Impact:           fmt.Sprintf("Reclaims %d MB of disk space and improves write performance on %s.", sizeMB, table),
			SQLFix:           fmt.Sprintf("DROP INDEX CONCURRENTLY %s.%s;", schema, index),
		})
	}
}
