package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	connectionsWeight = 0.08 // 8% of overall score
)

const queryConnections = `
SELECT 
    COUNT(*) AS total_conns,
    SUM(CASE WHEN state = 'active' THEN 1 ELSE 0 END) AS active_conns,
    SUM(CASE WHEN state = 'idle' THEN 1 ELSE 0 END) AS idle_conns,
    SUM(CASE WHEN state = 'idle in transaction' THEN 1 ELSE 0 END) AS idle_in_txn_conns
FROM pg_stat_activity
WHERE backend_type = 'client backend'
`

const queryMaxConnections = `
SELECT current_setting('max_connections')::int
`

type ConnectionAnalyzer struct{}

func (a *ConnectionAnalyzer) Name() string        { return "connections" }
func (a *ConnectionAnalyzer) Description() string { return "Connection pool and idle state analysis" }
func (a *ConnectionAnalyzer) Weight() float64     { return connectionsWeight }

func (a *ConnectionAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	var total, active, idle, idleInTxn int
	err := pool.QueryRow(ctx, queryConnections).Scan(&total, &active, &idle, &idleInTxn)
	if err != nil {
		return nil, fmt.Errorf("connection analysis failed: %w", err)
	}

	var maxConns int
	err = pool.QueryRow(ctx, queryMaxConnections).Scan(&maxConns)
	if err != nil {
		maxConns = 100 // fallback
	}

	utilization := float64(total) / float64(maxConns)

	if utilization > 0.85 {
		score.Score -= 30
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Connection exhaustion risk",
			Severity:         models.SeverityCritical,
			Description:      fmt.Sprintf("Connection utilization is at %.0f%% (%d of %d max_connections).", utilization*100, total, maxConns),
			CurrentValue:     fmt.Sprintf("%d connections", total),
			RecommendedValue: "< 70% utilization",
			Impact:           "When max_connections is reached, new connections will be rejected causing application outages.",
			SQLFix:           "Implement a connection pooler like PgBouncer, or increase max_connections (requires restart).",
		})
	}

	if idleInTxn > 0 {
		score.Score -= 5 * float64(idleInTxn)
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Idle in transaction connections",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("Found %d connections idle in transaction. These block vacuum from cleaning up dead tuples.", idleInTxn),
			CurrentValue:     fmt.Sprintf("%d idle in txn", idleInTxn),
			RecommendedValue: "0",
			Impact:           "Can cause severe table bloat and transaction ID wraparound issues over time.",
			SQLFix:           "SET idle_in_transaction_session_timeout = '5min';\n-- Or fix the application code to commit/rollback promptly.",
		})
	}

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}
