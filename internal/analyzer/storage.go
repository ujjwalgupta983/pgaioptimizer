package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	storageWeight = 0.05 // 5% of overall score
)

const queryWALStats = `
SELECT 
    wal_records, 
    wal_fpi, 
    wal_bytes, 
    wal_buffers_full, 
    wal_write, 
    wal_sync, 
    wal_write_time, 
    wal_sync_time 
FROM pg_stat_wal
`

const queryLargeToast = `
SELECT 
    n.nspname AS schemaname,
    c.relname,
    pg_table_size(t.oid) AS toast_size,
    pg_total_relation_size(c.oid) AS total_size
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_class t ON c.reltoastrelid = t.oid
WHERE pg_table_size(t.oid) > 1073741824 -- 1GB
ORDER BY toast_size DESC
LIMIT 10
`

type StorageAnalyzer struct{}

func (a *StorageAnalyzer) Name() string        { return "storage" }
func (a *StorageAnalyzer) Description() string { return "Database size, TOAST, and WAL performance" }
func (a *StorageAnalyzer) Weight() float64     { return storageWeight }

func (a *StorageAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	// pg_stat_wal is only available in PG14+
	if info.VersionNum >= 140000 {
		a.analyzeWAL(ctx, pool, score)
	}

	a.analyzeTOAST(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *StorageAnalyzer) analyzeWAL(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	var records, fpi, bytes, buffersFull, write, sync int64
	var writeTime, syncTime float64

	err := pool.QueryRow(ctx, queryWALStats).Scan(&records, &fpi, &bytes, &buffersFull, &write, &sync, &writeTime, &syncTime)
	if err != nil {
		return
	}

	if buffersFull > 1000 {
		score.Score -= 10
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "WAL Buffers Filling Up",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("WAL buffers became full %d times. This forces PostgreSQL to pause writes to wait for WAL to flush to disk.", buffersFull),
			CurrentValue:     fmt.Sprintf("%d times full", buffersFull),
			RecommendedValue: "0 times",
			Impact:           "Slows down all insert/update/delete operations.",
			SQLFix:           "Increase wal_buffers in postgresql.conf (e.g., to 16MB).",
		})
	}
}

func (a *StorageAnalyzer) analyzeTOAST(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	rows, err := pool.Query(ctx, queryLargeToast)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		var toastSize, totalSize int64

		if err := rows.Scan(&schema, &table, &toastSize, &totalSize); err != nil {
			continue
		}

		toastMB := toastSize / (1024 * 1024)
		totalMB := totalSize / (1024 * 1024)
		toastRatio := float64(toastSize) / float64(totalSize)

		if toastRatio > 0.5 && toastMB > 1024 {
			score.Score -= 5
			score.Findings = append(score.Findings, models.Finding{
				Category:         a.Name(),
				Title:            fmt.Sprintf("Large TOAST table on %s.%s", schema, table),
				Severity:         models.SeverityInfo,
				Description:      fmt.Sprintf("Table has %d MB of TOAST data, which is %.1f%% of its total size (%d MB).", toastMB, toastRatio*100, totalMB),
				CurrentValue:     fmt.Sprintf("%d MB TOAST", toastMB),
				RecommendedValue: "Avoid storing large binary/text objects in database if possible",
				Impact:           "Increases database size, backup times, and can cause bloat.",
				SQLFix:           "-- Investigate if large JSON/TEXT/BYTEA columns can be offloaded to object storage (e.g., S3).",
			})
		}
	}
}
