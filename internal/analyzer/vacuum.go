package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	vacuumWeight = 0.10 // 10% of overall score

	// Thresholds
	deadTupleRatioWarning  = 0.10 // 10%
	deadTupleRatioCritical = 0.20 // 20%
)

const queryDeadTuples = `
SELECT 
    schemaname, 
    relname, 
    n_live_tup, 
    n_dead_tup,
    CASE 
        WHEN n_live_tup + n_dead_tup > 0 
        THEN n_dead_tup::float / (n_live_tup + n_dead_tup) 
        ELSE 0 
    END AS dead_ratio
FROM pg_stat_user_tables
WHERE n_live_tup + n_dead_tup > 1000
ORDER BY dead_ratio DESC
`

type VacuumAnalyzer struct{}

func (a *VacuumAnalyzer) Name() string        { return "vacuum" }
func (a *VacuumAnalyzer) Description() string { return "Autovacuum health and dead tuple analysis" }
func (a *VacuumAnalyzer) Weight() float64     { return vacuumWeight }

func (a *VacuumAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	rows, err := pool.Query(ctx, queryDeadTuples)
	if err != nil {
		return nil, fmt.Errorf("vacuum analysis failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		var liveTup, deadTup int64
		var deadRatio float64

		if err := rows.Scan(&schema, &table, &liveTup, &deadTup, &deadRatio); err != nil {
			continue
		}

		if deadRatio > deadTupleRatioCritical {
			score.Score -= 10
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("Critical dead tuple ratio on %s.%s", schema, table),
				Severity:         models.SeverityCritical,
				Description:      fmt.Sprintf("Table has %.1f%% dead tuples (%d dead / %d total).", deadRatio*100, deadTup, liveTup+deadTup),
				CurrentValue:     fmt.Sprintf("%.1f%% dead tuples", deadRatio*100),
				RecommendedValue: "< 5%",
				Impact:           "High bloat increases scan time, wastes storage, and can cause unpredictable query performance.",
				SQLFix:           fmt.Sprintf("VACUUM ANALYZE %s.%s;\n-- If bloat is severe, consider VACUUM FULL (requires exclusive lock) or pg_repack.", schema, table),
			})
		} else if deadRatio > deadTupleRatioWarning {
			score.Score -= 5
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("High dead tuple ratio on %s.%s", schema, table),
				Severity:         models.SeverityWarning,
				Description:      fmt.Sprintf("Table has %.1f%% dead tuples (%d dead / %d total).", deadRatio*100, deadTup, liveTup+deadTup),
				CurrentValue:     fmt.Sprintf("%.1f%% dead tuples", deadRatio*100),
				RecommendedValue: "< 5%",
				Impact:           "Increases sequential scan time and disk usage.",
				SQLFix:           fmt.Sprintf("VACUUM ANALYZE %s.%s;", schema, table),
			})
		}
	}

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}
