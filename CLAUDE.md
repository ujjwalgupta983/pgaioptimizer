# pgaioptimizer — Agent Context

## Project Overview

pgaioptimizer is a PostgreSQL AI Performance Analyzer written in Go. It connects to PostgreSQL instances (managed or self-hosted), collects metrics from system catalog views, analyzes them, and produces a scored health report with actionable recommendations.

**Repo:** github.com/ujjwalgupta983/pgaioptimizer  
**Language:** Go 1.22+  
**Module:** github.com/ujjwalgupta983/pgaioptimizer

## Architecture Summary

- **Three-tier insight model:** Tier 1 (SQL-only, all PG), Tier 2 (Cloud API enrichment), Tier 3 (OS agent for self-hosted)
- **12 analysis categories:** configuration, indexes, tables, queries, connections, vacuum, cache, locks, sequences, storage, replication, schema
- **Storage:** DuckDB (columnar, embedded) for agent mode; in-memory for scan mode
- **Frontend:** Vite + React + TypeScript dashboard
- **Three deployment modes:** `scan` (one-shot), `agent` (continuous), `server` (multi-instance)

See [ARCHITECTURE.md](./ARCHITECTURE.md) for full details.

## Code Structure

```
cmd/pgaioptimizer/main.go       — CLI entrypoint (cobra)
internal/
  collector/
    sql.go                       — Tier 1: pg_stat_* SQL queries
    cloud/aws.go|gcp.go|azure.go — Tier 2: cloud API metrics
    os/cpu.go|memory.go|disk.go  — Tier 3: /proc/* OS metrics
  analyzer/
    analyzer.go                  — Base Analyzer interface
    configuration.go             — pg_settings analysis
    indexes.go                   — Index health (unused, duplicate, missing)
    tables.go                    — Table stats (dead tuples, bloat, seq scans)
    queries.go                   — pg_stat_statements analysis
    connections.go               — pg_stat_activity analysis
    vacuum.go                    — Autovacuum health + txid wraparound
    cache.go                     — Cache hit ratio + bgwriter
    locks.go                     — Lock conflicts + blocking chains
    sequences.go                 — Sequence exhaustion
    storage.go                   — Database/table sizes + WAL
    replication.go               — Replica lag + slots
    schema.go                    — Schema design suggestions
  engine/
    scoring.go                   — Weighted health score (0-100)
    recommendations.go           — Priority sorting + SQL fix generation
    correlator.go                — Cross-category root cause analysis
  models/
    finding.go                   — Finding, Severity, CategoryScore, HealthReport
  storage/
    duckdb.go                    — DuckDB snapshot storage
  api/
    server.go                    — REST API (gin/echo)
  report/
    html.go                      — HTML report generator
web/                             — Vite + React frontend
```

## Key Conventions

### Go Code Style
- Use `context.Context` as first parameter for all DB/network operations
- Use `pgx/v5` for PostgreSQL connections (not `database/sql`)
- Use `pgxpool.Pool` for connection pooling (pass pool, not individual connections)
- Error wrapping: `fmt.Errorf("action failed: %w", err)`
- All SQL queries must be read-only (`SELECT` only, never modify target DB)
- Use constants for threshold values, not magic numbers

### Analyzer Pattern
Every analyzer implements the `Analyzer` interface:
```go
type Analyzer interface {
    Name() string
    Description() string
    Weight() float64
    Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error)
}
```

Each analyzer: collect stats → evaluate rules → compute score → generate findings.

### Finding Structure
```go
Finding{
    Category:         "indexes",
    Title:            "Unused index detected",
    Severity:         SeverityWarning,
    Description:      "Human-readable explanation",
    CurrentValue:     "idx_scan = 0 (7 days of stats)",
    RecommendedValue: "Drop or investigate",
    Impact:           "Saves X MB, improves write perf by Y%",
    SQLFix:           "DROP INDEX CONCURRENTLY idx_name;",
}
```

### SQL Queries
- Always use parameterized queries (`$1`, `$2`) to prevent injection
- Handle PG version differences: check `ServerInfo.VersionNum` before using PG14+/PG16+ features
- Handle missing extensions gracefully (check `pg_extension` before querying `pg_stat_statements`)
- All collection queries should complete in <100ms individually

### Testing
- Unit tests: mock `pgxpool.Pool` with `pgxmock`
- Integration tests: use Docker PostgreSQL via `testcontainers-go`
- Run: `make test`

### Dependencies
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/spf13/cobra` — CLI framework
- `github.com/spf13/viper` — Configuration
- `github.com/marcboeker/go-duckdb` — DuckDB embedded storage
- No ORM. Raw SQL only.

## Important Rules

1. **Never write to the target PostgreSQL database.** All operations are SELECT-only.
2. **Never access user data.** We query `pg_stat_*` views and `pg_catalog`, never user tables.
3. **Always handle missing features gracefully.** If `pg_stat_statements` isn't installed, skip query analysis with an INFO finding recommending installation.
4. **Every finding must include a confidence level.** HIGH (>90%), MEDIUM (70-90%), LOW (50-70%).
5. **Every SQL fix must be safe.** Use `CONCURRENTLY` for index operations. Warn about locks for `VACUUM FULL`. Never suggest destructive operations without explicit warnings.
6. **Performance budget:** Full scan must complete in <10 seconds. Individual collector query <100ms.
