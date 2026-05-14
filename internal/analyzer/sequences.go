package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	sequencesWeight = 0.03 // 3% of overall score
)

const querySequences = `
SELECT 
    schemaname, 
    sequencename, 
    data_type, 
    last_value, 
    max_value 
FROM pg_sequences
`

type SequenceAnalyzer struct{}

func (a *SequenceAnalyzer) Name() string        { return "sequences" }
func (a *SequenceAnalyzer) Description() string { return "Sequence exhaustion risk analysis" }
func (a *SequenceAnalyzer) Weight() float64     { return sequencesWeight }

func (a *SequenceAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeSequences(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *SequenceAnalyzer) analyzeSequences(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, querySequences)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, sequence, dataType string
		var lastValue, maxValue int64

		if err := rows.Scan(&schema, &sequence, &dataType, &lastValue, &maxValue); err != nil {
			continue
		}

		if maxValue == 0 {
			continue
		}

		exhaustionRatio := float64(lastValue) / float64(maxValue)

		if exhaustionRatio > 0.90 {
			score.Score -= 50
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("Critical: Sequence %s.%s almost exhausted", schema, sequence),
				Severity:         models.SeverityCritical,
				Description:      fmt.Sprintf("Sequence is at %.1f%% of its maximum capacity for type %s.", exhaustionRatio*100, dataType),
				CurrentValue:     fmt.Sprintf("%d of %d", lastValue, maxValue),
				RecommendedValue: "Change data type to BIGINT",
				Impact:           "If the sequence reaches max_value, inserts into the dependent table will fail entirely causing a major outage.",
				SQLFix:           fmt.Sprintf("ALTER SEQUENCE %s.%s AS bigint;\n-- Note: You must also ALTER TABLE the referencing column to bigint.", schema, sequence),
			})
		} else if exhaustionRatio > 0.70 {
			score.Score -= 10
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("Warning: Sequence %s.%s approaching limit", schema, sequence),
				Severity:         models.SeverityWarning,
				Description:      fmt.Sprintf("Sequence is at %.1f%% of its maximum capacity for type %s.", exhaustionRatio*100, dataType),
				CurrentValue:     fmt.Sprintf("%d of %d", lastValue, maxValue),
				RecommendedValue: "Change data type to BIGINT",
				Impact:           "If the sequence reaches max_value, inserts into the dependent table will fail.",
				SQLFix:           fmt.Sprintf("ALTER SEQUENCE %s.%s AS bigint;\n-- Note: You must also ALTER TABLE the referencing column to bigint.", schema, sequence),
			})
		}
	}
}
