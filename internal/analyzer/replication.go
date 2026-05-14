package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	replicationWeight = 0.02 // 2% of overall score
)

const queryReplicationSlots = `
SELECT 
    slot_name, 
    plugin, 
    slot_type, 
    active, 
    pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn) AS retained_bytes
FROM pg_replication_slots
`

const queryReplicationStat = `
SELECT 
    client_addr, 
    state, 
    pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) AS lag_bytes
FROM pg_stat_replication
`

type ReplicationAnalyzer struct{}

func (a *ReplicationAnalyzer) Name() string        { return "replication" }
func (a *ReplicationAnalyzer) Description() string { return "Replication slots and replica lag" }
func (a *ReplicationAnalyzer) Weight() float64     { return replicationWeight }

func (a *ReplicationAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeSlots(ctx, pool, score)
	a.analyzeLag(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *ReplicationAnalyzer) analyzeSlots(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryReplicationSlots)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var slotName, slotType string
		var plugin *string
		var active bool
		var retainedBytes *int64

		if err := rows.Scan(&slotName, &plugin, &slotType, &active, &retainedBytes); err != nil {
			continue
		}

		if !active && retainedBytes != nil {
			retainedMB := *retainedBytes / (1024 * 1024)

			// If inactive slot is retaining >1GB of WAL
			if retainedMB > 1024 {
				score.Score -= 20
				score.Findings = append(score.Findings, models.Finding{
					Category:         a.Name(),
					Title:            fmt.Sprintf("Inactive replication slot: %s", slotName),
					Severity:         models.SeverityCritical,
					Description:      fmt.Sprintf("Slot is inactive and is preventing PostgreSQL from removing %d MB of old WAL files.", retainedMB),
					CurrentValue:     fmt.Sprintf("%d MB WAL retained", retainedMB),
					RecommendedValue: "0 MB",
					Impact:           "Can cause the primary database to run out of disk space and crash.",
					SQLFix:           fmt.Sprintf("SELECT pg_drop_replication_slot('%s');", slotName),
				})
			}
		}
	}
}

func (a *ReplicationAnalyzer) analyzeLag(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryReplicationStat)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var clientAddr, state string
		var lagBytes *int64

		if err := rows.Scan(&clientAddr, &state, &lagBytes); err != nil {
			continue
		}

		if lagBytes != nil {
			lagMB := *lagBytes / (1024 * 1024)

			if lagMB > 500 {
				score.Score -= 10
				score.Findings = append(score.Findings, models.Finding{
					Category:         a.Name(),
					Title:            fmt.Sprintf("High replication lag for %s", clientAddr),
					Severity:         models.SeverityWarning,
					Description:      fmt.Sprintf("Replica is lagging behind by %d MB of WAL.", lagMB),
					CurrentValue:     fmt.Sprintf("%d MB lag", lagMB),
					RecommendedValue: "< 10 MB",
					Impact:           "Read queries on the replica may return stale data, and failovers will take longer.",
					SQLFix:           "-- Investigate network bandwidth or replica I/O performance.",
				})
			}
		}
	}
}
