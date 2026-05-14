package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	schemaWeight = 0.02 // 2% of overall score
)

const queryMissingPK = `
SELECT 
    schemaname, 
    relname, 
    n_live_tup
FROM pg_stat_user_tables
WHERE relid NOT IN (
    SELECT indrelid FROM pg_index WHERE indisprimary
)
ORDER BY n_live_tup DESC
`

type SchemaAnalyzer struct{}

func (a *SchemaAnalyzer) Name() string        { return "schema" }
func (a *SchemaAnalyzer) Description() string { return "Schema design and best practices" }
func (a *SchemaAnalyzer) Weight() float64     { return schemaWeight }

func (a *SchemaAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeMissingPK(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *SchemaAnalyzer) analyzeMissingPK(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryMissingPK)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		var liveTup int64

		if err := rows.Scan(&schema, &table, &liveTup); err != nil {
			continue
		}

		if liveTup > 1000 {
			score.Score -= 5
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("Table missing primary key: %s.%s", schema, table),
				Severity:         models.SeverityInfo,
				Description:      fmt.Sprintf("Table has %d rows but no primary key defined.", liveTup),
				CurrentValue:     "No PK",
				RecommendedValue: "Define a Primary Key",
				Impact:           "Tables without primary keys cannot be logically replicated and may suffer from duplicate rows.",
				SQLFix:           fmt.Sprintf("ALTER TABLE %s.%s ADD PRIMARY KEY (id);", schema, table),
			})
		}
	}
}
