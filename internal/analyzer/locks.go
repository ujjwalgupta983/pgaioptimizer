package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	locksWeight = 0.05 // 5% of overall score
)

const queryBlockingLocks = `
SELECT
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocked_activity.query AS blocked_query,
    blocking_activity.query AS blocking_query
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks 
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.DATABASE IS NOT DISTINCT FROM blocked_locks.DATABASE
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted
LIMIT 50
`

const queryDeadlocks = `
SELECT sum(deadlocks) AS total_deadlocks FROM pg_stat_database
`

type LockAnalyzer struct{}

func (a *LockAnalyzer) Name() string        { return "locks" }
func (a *LockAnalyzer) Description() string { return "Database locking and blocking chains" }
func (a *LockAnalyzer) Weight() float64     { return locksWeight }

func (a *LockAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeBlockingLocks(ctx, pool, score)
	a.analyzeDeadlocks(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *LockAnalyzer) analyzeBlockingLocks(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryBlockingLocks)
	if err != nil {
		return
	}
	defer rows.Close()

	blockedCount := 0
	for rows.Next() {
		blockedCount++
	}

	if blockedCount > 0 {
		score.Score -= float64(10 * blockedCount) // Severe penalty for active blocking locks
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Active Blocking Locks Detected",
			Severity:         models.SeverityCritical,
			Description:      fmt.Sprintf("Detected %d blocked processes. Some queries are unable to proceed because another transaction holds a conflicting lock.", blockedCount),
			CurrentValue:     fmt.Sprintf("%d blocked queries", blockedCount),
			RecommendedValue: "0 blocked queries",
			Impact:           "Severe degradation of application concurrency and throughput.",
			SQLFix:           "Review the blocked and blocking queries. Consider terminating the blocking process: SELECT pg_terminate_backend(<blocking_pid>);",
		})
	}
}

func (a *LockAnalyzer) analyzeDeadlocks(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	var totalDeadlocks int64
	err := pool.QueryRow(ctx, queryDeadlocks).Scan(&totalDeadlocks)
	if err != nil {
		return
	}

	if totalDeadlocks > 0 {
		score.Score -= 10
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Historical Deadlocks Detected",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("Detected %d deadlocks historically on this instance.", totalDeadlocks),
			CurrentValue:     fmt.Sprintf("%d deadlocks", totalDeadlocks),
			RecommendedValue: "0 deadlocks",
			Impact:           "Transactions were forcefully aborted by PostgreSQL to resolve a deadlock.",
			SQLFix:           "Ensure application transactions acquire locks on multiple objects in a consistent, predictable order.",
		})
	}
}
