# Agent Skills

## Skill: Write a New Analyzer

When creating a new analysis module (e.g., `internal/analyzer/vacuum.go`):

1. Create the file implementing the `Analyzer` interface
2. Define threshold constants at the top of the file
3. Write collection SQL queries as const strings
4. Implement `Analyze()` following this pattern:

```go
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
    vacuumAgeDaysWarning   = 7
)

const queryDeadTuples = `
SELECT schemaname, relname, 
    n_live_tup, n_dead_tup,
    CASE WHEN n_live_tup + n_dead_tup > 0 
         THEN n_dead_tup::float / (n_live_tup + n_dead_tup) 
         ELSE 0 END AS dead_ratio,
    last_autovacuum, last_vacuum
FROM pg_stat_user_tables
WHERE n_live_tup + n_dead_tup > 1000
ORDER BY dead_ratio DESC
`

type VacuumAnalyzer struct{}

func (a *VacuumAnalyzer) Name() string        { return "vacuum" }
func (a *VacuumAnalyzer) Description() string  { return "Autovacuum health and dead tuple analysis" }
func (a *VacuumAnalyzer) Weight() float64      { return vacuumWeight }

func (a *VacuumAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
    score := &models.CategoryScore{
        Category:    a.Name(),
        Score:       100,
        Weight:      a.Weight(),
        Description: a.Description(),
    }

    // 1. Collect
    rows, err := pool.Query(ctx, queryDeadTuples)
    if err != nil {
        return nil, fmt.Errorf("vacuum analysis failed: %w", err)
    }
    defer rows.Close()

    // 2. Analyze + 3. Generate findings
    for rows.Next() {
        var schema, table string
        var liveTup, deadTup int64
        var deadRatio float64
        // ... scan and evaluate thresholds
        
        if deadRatio > deadTupleRatioCritical {
            score.Score -= 30
            score.Findings = append(score.Findings, models.Finding{
                Category:     a.Name(),
                Title:        fmt.Sprintf("Critical dead tuple ratio on %s.%s", schema, table),
                Severity:     models.SeverityCritical,
                Description:  fmt.Sprintf("Table has %.1f%% dead tuples (%d dead / %d total)", deadRatio*100, deadTup, liveTup+deadTup),
                CurrentValue: fmt.Sprintf("%.1f%% dead tuples", deadRatio*100),
                RecommendedValue: "< 5%",
                Impact:       "High bloat increases scan time and wastes storage",
                SQLFix:       fmt.Sprintf("VACUUM ANALYZE %s.%s;", schema, table),
            })
        }
    }

    // Floor at 0
    if score.Score < 0 { score.Score = 0 }
    return score, nil
}
```

4. Register in `analyzer/analyzer.go` → `NewRegistry()`
5. Write unit test in `analyzer/vacuum_test.go`

---

## Skill: Write a Collection Query

SQL queries for data collection must follow these rules:

```go
// GOOD: Version-aware, parameterized, read-only
const queryWALStats = `
SELECT wal_records, wal_bytes, wal_buffers_full,
       wal_write_time, wal_sync_time
FROM pg_stat_wal
`

func collectWAL(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*WALStats, error) {
    // Check version compatibility
    if info.VersionNum < 140000 {
        return nil, nil // pg_stat_wal requires PG14+, skip gracefully
    }
    
    var stats WALStats
    err := pool.QueryRow(ctx, queryWALStats).Scan(
        &stats.Records, &stats.Bytes, &stats.BuffersFull,
        &stats.WriteTime, &stats.SyncTime,
    )
    if err != nil {
        return nil, fmt.Errorf("WAL stats collection failed: %w", err)
    }
    return &stats, nil
}
```

Key rules:
- SQL as `const` strings (not built dynamically)
- Check `info.VersionNum` for version-specific views (140000 = PG14, 160000 = PG16)
- Return `nil, nil` (not error) for missing features — the analyzer handles absence
- Use `pool.QueryRow` for single-row results, `pool.Query` for multi-row
- Always `defer rows.Close()` for multi-row queries

---

## Skill: Add a Correlation Rule

Correlation rules connect findings from different categories:

```go
// In engine/correlator.go
type CorrelationRule struct {
    Name       string
    Categories []string // which categories must have findings
    Evaluate   func(findings map[string][]models.Finding) *models.Finding
}

var cacheMissRootCause = CorrelationRule{
    Name:       "cache_miss_root_cause",
    Categories: []string{"cache", "configuration", "tables"},
    Evaluate: func(findings map[string][]models.Finding) *models.Finding {
        hasLowCache := findBySeverity(findings["cache"], models.SeverityWarning)
        hasSmallBuffers := findByTitle(findings["configuration"], "shared_buffers")
        hasSeqScans := findByTitle(findings["tables"], "sequential scan")
        
        if hasLowCache != nil && hasSmallBuffers != nil && hasSeqScans != nil {
            return &models.Finding{
                Category:    "correlation",
                Title:       "Low cache hit ratio caused by undersized buffers + missing indexes",
                Severity:    models.SeverityCritical,
                Description: "These three findings share a root cause...",
                // ...
            }
        }
        return nil
    },
}
```

---

## Skill: DuckDB Storage Operations

```go
// Write a snapshot
func (s *Store) WriteSnapshot(ctx context.Context, snap *Snapshot) error {
    tx, _ := s.db.BeginTx(ctx, nil)
    defer tx.Rollback()
    
    // Use DuckDB's batch insert for table metrics (much faster than row-by-row)
    appender, _ := duckdb.NewAppenderFromConn(tx, "", "m_tables")
    defer appender.Close()
    
    for _, t := range snap.Tables {
        appender.AppendRow(snap.ID, snap.TakenAt, t.Schema, t.Name, ...)
    }
    appender.Flush()
    tx.Commit()
}

// Query with time range (columnar = fast aggregation)
func (s *Store) CacheHitRatioTrend(ctx context.Context, since time.Time) ([]DataPoint, error) {
    rows, _ := s.db.QueryContext(ctx, `
        SELECT 
            time_bucket(INTERVAL '5 minutes', taken_at) AS bucket,
            AVG(blks_hit::DOUBLE / NULLIF(blks_hit + blks_read, 0)) AS ratio
        FROM m_database
        WHERE taken_at >= ?
        GROUP BY bucket
        ORDER BY bucket
    `, since)
    // ...
}
```

---

## Skill: Frontend Component Pattern

```tsx
// React component for a category card
interface CategoryCardProps {
    category: CategoryScore;
    onClick: () => void;
}

function CategoryCard({ category, onClick }: CategoryCardProps) {
    const color = category.score >= 80 ? '#10b981' 
                : category.score >= 60 ? '#f59e0b' 
                : '#ef4444';
    
    return (
        <div className="category-card" onClick={onClick}>
            <ScoreGauge score={category.score} color={color} size={64} />
            <h3>{category.category}</h3>
            <div className="finding-badges">
                {category.findings.filter(f => f.severity === 'critical').length > 0 && (
                    <span className="badge critical">
                        {category.findings.filter(f => f.severity === 'critical').length} critical
                    </span>
                )}
            </div>
        </div>
    );
}
```
