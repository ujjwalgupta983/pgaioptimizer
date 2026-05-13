# Agent Memory — pgaioptimizer

This file serves as persistent memory for AI coding agents working on this project. Update this file as decisions are made and context evolves.

## Project Decisions Log

### 2026-05-13: Project Inception
- **Decision:** Build pgaioptimizer as a Go-based PostgreSQL performance analyzer
- **Rationale:** Go chosen for single-binary distribution, low memory footprint for agent mode, cross-compilation
- **Alternatives considered:** Python (better analysis ecosystem but poor for agents), TypeScript (familiar to team but wrong tool)

### 2026-05-13: Storage Layer
- **Decision:** DuckDB (columnar, embedded) over SQLite
- **Rationale:** Our query patterns are analytical (aggregations, time-ranges, trend computation). DuckDB is 10-100x faster for these. SQLite is OLTP-optimized (row-based), wrong for time-series metrics.
- **Tradeoff:** DuckDB Go bindings require CGO → complicates cross-compilation. Mitigated by providing pre-built binaries via GoReleaser.

### 2026-05-13: Three-Tier Insight Model
- **Decision:** Tier 1 (SQL-only) → Tier 2 (Cloud API) → Tier 3 (Agent + OS + extensions)
- **Rationale:** Maximizes compatibility. Tier 1 works on any PG including RDS. Tier 3 provides dramatically deeper insights for self-hosted PG where extensions and OS access are available.

## Current Architecture State

| Component | Status | Notes |
|:---|:---|:---|
| CLI skeleton | ✅ Created | `cmd/pgaioptimizer/main.go` — needs cobra integration |
| Core models | ✅ Created | `internal/models/finding.go` |
| Analyzer interface | ✅ Created | `internal/analyzer/analyzer.go` |
| SQL collector | ✅ Created | `internal/collector/sql.go` |
| ARCHITECTURE.md | ✅ Complete | Full diagrams, storage, analysis engine, prediction model |
| Go module init | ❌ Pending | Need `go mod init` (Go not yet installed) |
| 12 analyzers | ❌ Pending | Phase 2-3 |
| Scoring engine | ❌ Pending | Phase 4 |
| DuckDB storage | ❌ Pending | Phase 5 |
| Frontend | ❌ Pending | Phase 6 |
| Cloud APIs | ❌ Pending | Phase 7 |

## Key Technical Context

### PostgreSQL Version Compatibility
- PG12+: baseline support (all pg_stat_* views)
- PG14+: `pg_stat_wal` view available
- PG16+: `pg_stat_io` view available (detailed I/O by backend type)
- Always check `ServerInfo.VersionNum` before using version-specific features

### RDS-Specific Notes
- `pg_stat_statements` is supported but must be enabled in Parameter Group
- `pg_stat_kcache`, `pg_qualstats`, `pg_wait_sampling`, `HypoPG` are NOT available on RDS
- User role is `rds_superuser` (not true superuser) — most stat views are readable
- `track_io_timing` must be enabled in Parameter Group for I/O timing data

### Self-Hosted Exclusive Extensions
- `pg_stat_kcache`: per-query kernel CPU + real disk I/O
- `pg_qualstats`: WHERE clause predicate statistics → enables smart index suggestions
- `pg_wait_sampling`: wait event profiling per query
- `HypoPG`: hypothetical index creation and cost estimation
- `pg_buffercache`: inspect shared_buffers contents
- `pgstattuple`: exact bloat measurement

### Scoring Weights
```
configuration: 15%  |  indexes: 15%     |  queries: 15%
tables: 10%         |  cache: 10%       |  vacuum: 10%
connections: 8%     |  locks: 5%        |  storage: 5%
sequences: 3%       |  replication: 2%  |  schema: 2%
```

## Open Issues / TODO

- [ ] Install Go on dev machine (`brew install go`)
- [ ] Run `go mod init github.com/ujjwalgupta983/pgaioptimizer`
- [ ] Integrate cobra CLI framework
- [ ] Decide on DuckDB Go binding: `github.com/marcboeker/go-duckdb` vs `github.com/motherduck-com/go-duckdb`
- [ ] Design the React dashboard mockup before building
- [ ] Decide on AI provider for correlation engine (OpenAI vs local model vs rule-based only)

## Files Changed Tracker

Track files modified in each session to avoid conflicts:

### Session 2026-05-13
- Created: `cmd/pgaioptimizer/main.go`
- Created: `internal/models/finding.go`
- Created: `internal/analyzer/analyzer.go`
- Created: `internal/collector/sql.go`
- Created: `ARCHITECTURE.md`
- Created: `README.md`
- Created: `Makefile`
- Created: `CLAUDE.md`
- Created: `.gemini/skills.md`
