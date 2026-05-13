# Architecture

## End-to-End Data Flow

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  1. CONNECT  │───▶│  2. COLLECT   │───▶│  3. STORE    │───▶│  4. ANALYZE  │───▶│  5. REPORT   │
│              │    │              │    │              │    │              │    │              │
│ Target PG    │    │ SQL queries  │    │ In-memory or │    │ Rule engine  │    │ Scored report│
│ connection   │    │ OS metrics   │    │ SQLite       │    │ Correlation  │    │ with fixes   │
│ validation   │    │ Cloud APIs   │    │ snapshots    │    │ Prediction   │    │ and estimates│
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
```

### Step 1: Connect
- Parse DSN or individual host/port/db/user/password
- Establish connection pool via `pgx` (3 connections default)
- Validate connectivity with `SELECT 1`
- Detect: PG version, superuser status, installed extensions, server OS

### Step 2: Collect
- Run 60+ diagnostic SQL queries against `pg_stat_*` views
- All queries are **read-only** (`SELECT` only, no temp tables, no functions)
- Tier 3 (agent): also read `/proc/*` for OS metrics
- Tier 2 (cloud): also call CloudWatch/GCP/Azure APIs
- Entire collection phase takes 2-5 seconds

### Step 3: Store
- **Scan mode**: all data stays in-memory structs, discarded after report
- **Agent mode**: snapshots written to local SQLite every N seconds
- Delta computation for rate-based metrics (queries/sec, blocks/sec)

### Step 4: Analyze
- Each of 12 analyzers processes its relevant data
- Rule engine evaluates thresholds, formulas, and patterns
- Cross-category correlator links related findings
- Scoring engine computes weighted health score
- Prediction engine estimates improvement impact

### Step 5: Report
- Findings sorted by severity (critical → info)
- Each finding includes: description, current value, recommended value, SQL fix, estimated impact
- Output as HTML report, JSON, or served via web dashboard

---

## Storage Architecture

### Scan Mode (stateless, in-memory)

No persistent storage. Data flows through Go structs:

```
CollectedStats {
  Database:    pg_stat_database rows
  Tables:      pg_stat_user_tables rows
  Indexes:     pg_stat_user_indexes rows
  Queries:     pg_stat_statements rows (if available)
  Activity:    pg_stat_activity rows
  BGWriter:    pg_stat_bgwriter row
  WAL:         pg_stat_wal row (PG14+)
  IO:          pg_stat_io rows (PG16+)
  Settings:    pg_settings rows
  Locks:       pg_locks rows
  Sequences:   pg_sequences rows
  Replication: pg_stat_replication rows
  Schema:      information_schema rows
  OSMetrics:   /proc/* data (Tier 3 only)
}
```

### Agent Mode (SQLite, time-series snapshots)

```sql
-- Core snapshot tracking
CREATE TABLE snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    taken_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    pg_version  TEXT,
    duration_ms INTEGER  -- how long collection took
);

-- Database-level metrics (one row per snapshot)
CREATE TABLE metric_database (
    snapshot_id     INTEGER REFERENCES snapshots(id),
    xact_commit     BIGINT,
    xact_rollback   BIGINT,
    blks_read       BIGINT,
    blks_hit        BIGINT,
    tup_returned    BIGINT,
    tup_fetched     BIGINT,
    tup_inserted    BIGINT,
    tup_updated     BIGINT,
    tup_deleted     BIGINT,
    deadlocks       BIGINT,
    temp_files       BIGINT,
    temp_bytes       BIGINT,
    cache_hit_ratio REAL   -- computed: blks_hit / (blks_hit + blks_read)
);

-- Table-level metrics (one row per table per snapshot)
CREATE TABLE metric_tables (
    snapshot_id    INTEGER REFERENCES snapshots(id),
    schema_name    TEXT,
    table_name     TEXT,
    seq_scan       BIGINT,
    idx_scan       BIGINT,
    n_live_tup     BIGINT,
    n_dead_tup     BIGINT,
    last_vacuum    TIMESTAMP,
    last_autovacuum TIMESTAMP,
    last_analyze   TIMESTAMP,
    table_size     BIGINT,
    dead_ratio     REAL    -- computed
);

-- Query metrics (top N queries per snapshot)
CREATE TABLE metric_queries (
    snapshot_id     INTEGER REFERENCES snapshots(id),
    query_hash      TEXT,   -- queryid from pg_stat_statements
    query_text      TEXT,
    calls           BIGINT,
    total_exec_time REAL,
    mean_exec_time  REAL,
    shared_blks_read  BIGINT,
    shared_blks_hit   BIGINT,
    temp_blks_written BIGINT,
    cache_hit_ratio   REAL
);

-- OS metrics (Tier 3 only)
CREATE TABLE metric_os (
    snapshot_id    INTEGER REFERENCES snapshots(id),
    cpu_user       REAL,
    cpu_system     REAL,
    cpu_iowait     REAL,
    mem_total      BIGINT,
    mem_available  BIGINT,
    mem_cached     BIGINT,
    disk_read_iops  REAL,
    disk_write_iops REAL,
    disk_read_bytes BIGINT,
    disk_write_bytes BIGINT
);

-- Historical reports
CREATE TABLE reports (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    generated_at TIMESTAMP NOT NULL,
    overall_score REAL,
    grade        TEXT,
    report_json  TEXT  -- full HealthReport as JSON
);
```

**Retention policy**: Default 30 days. Configurable. Old snapshots auto-pruned.

**Delta computation**: For cumulative counters (like `blks_read`), rates are computed as:
```
rate = (current_value - previous_value) / time_delta_seconds
```

---

## Analysis Engine

### Rule Types

The analysis engine uses four types of rules, from simple to complex:

#### Type 1: Threshold Rules
Compare a single metric against a known-good range.

```
Rule: cache_hit_ratio
  IF value >= 99%  → OK,    score_impact: 0
  IF value >= 95%  → OK,    score_impact: -5
  IF value >= 90%  → INFO,  score_impact: -15
  IF value >= 80%  → WARN,  score_impact: -30
  IF value <  80%  → CRIT,  score_impact: -50
```

Examples:
| Metric | OK | WARNING | CRITICAL |
|:---|:---|:---|:---|
| Cache hit ratio | ≥95% | 80-95% | <80% |
| Dead tuple ratio | <5% | 5-20% | >20% |
| Connection utilization | <70% | 70-90% | >90% |
| Sequence exhaustion | <50% | 50-80% | >80% |

#### Type 2: Formula Rules
Compute a recommended value from system parameters, compare to actual.

```
Rule: shared_buffers
  recommended = total_ram * 0.25
  actual = current_setting('shared_buffers')
  IF actual < recommended * 0.6  → WARNING ("shared_buffers is too low")
  IF actual > recommended * 1.6  → INFO ("shared_buffers may be too high")

Rule: work_mem
  recommended = (total_ram - shared_buffers) / (3 * max_connections)
  actual = current_setting('work_mem')
  IF actual == '4MB' (default) → WARNING ("work_mem is at default")
  IF actual < recommended * 0.5 → WARNING ("work_mem is too low")
```

#### Type 3: Pattern Rules
Detect multi-condition patterns in the data.

```
Rule: missing_index
  FOR EACH table WHERE:
    seq_scan > 1000
    AND n_live_tup > 100000
    AND seq_scan > (idx_scan * 10)  -- 10x more seq scans than index scans
  → WARNING "Table {name} has {seq_scan} sequential scans on {rows} rows"
  → SQL_FIX: (suggest index based on pg_qualstats data in Tier 3,
              or general advice in Tier 1)

Rule: unused_index
  FOR EACH index WHERE:
    idx_scan = 0
    AND NOT indisunique
    AND stats_age > 7 days  -- ensure stats are meaningful
  → INFO "Index {name} has never been used"
  → SQL_FIX: "DROP INDEX CONCURRENTLY {name};"

Rule: idle_in_transaction
  FOR EACH connection WHERE:
    state = 'idle in transaction'
    AND state_change < now() - interval '5 minutes'
  → WARNING "Connection {pid} idle in transaction for {duration}"
```

#### Type 4: Correlation Rules
Connect findings across categories to identify root causes.

```
Rule: cache_miss_root_cause
  IF cache_hit_ratio < 90%
    AND shared_buffers < total_ram * 0.15
    AND EXISTS(tables with high seq_scan count)
  → FINDING:
    title: "Low cache hit ratio caused by undersized shared_buffers and missing indexes"
    description: "Your cache hit ratio is {ratio}% because shared_buffers is only
                  {current} (should be ~{recommended}), AND {count} tables are doing
                  full sequential scans that don't fit in cache."
    sql_fix: [
      "ALTER SYSTEM SET shared_buffers = '{recommended}';",
      "CREATE INDEX CONCURRENTLY ... ON {table} ({columns});",
      "SELECT pg_reload_conf();"
    ]
    impact: "Combined fix could improve query response time by 5-20x"

Rule: work_mem_spillover
  IF EXISTS(queries with temp_blks_written > 0)
    AND work_mem <= '4MB'
  → FINDING:
    title: "Queries spilling to disk due to default work_mem"
    description: "{count} queries are writing temporary files because work_mem
                  is at the default 4MB. These queries sorted/hashed {total_temp}
                  of data to disk instead of memory."
    sql_fix: "ALTER SYSTEM SET work_mem = '{recommended}';"
    impact: "Eliminating disk spill could speed up affected queries by 3-10x"
```

### Analysis Flow per Category

```
┌─────────────────────────────────────────────────────────┐
│                    Analyzer Module                       │
│                                                         │
│  1. Collect:  Run category-specific SQL queries          │
│       │                                                 │
│  2. Evaluate: Apply rules (threshold → formula →        │
│       │        pattern → correlation)                   │
│       │                                                 │
│  3. Score:    Start at 100, subtract for each finding   │
│       │        CRITICAL: -20 to -40 per finding         │
│       │        WARNING:  -5 to -20 per finding          │
│       │        INFO:     -1 to -5 per finding           │
│       │        Floor at 0                               │
│       │                                                 │
│  4. Recommend: Generate Finding objects with SQL fixes   │
│       │         and estimated impact                    │
│       │                                                 │
│  5. Return:   CategoryScore { score, findings, weight } │
└─────────────────────────────────────────────────────────┘
```

### Overall Score Computation

```
overall_score = Σ (category_score × category_weight)

Where weights sum to 1.0:
  configuration: 0.15
  indexes:       0.15
  queries:       0.15
  tables:        0.10
  cache:         0.10
  vacuum:        0.10
  connections:   0.08
  locks:         0.05
  storage:       0.05
  sequences:     0.03
  replication:   0.02  (0 if no replicas, weight redistributed)
  schema:        0.02
```

---

## Prediction & Impact Estimation

### How We Estimate Improvements

For each recommendation, we estimate the performance impact. Here's how:

#### Index Recommendations

**Method**: Compare sequential scan cost vs index scan cost using PostgreSQL's cost model.

```
Sequential scan cost:
  cost_seq = seq_page_cost × pages + cpu_tuple_cost × rows

Index scan cost (btree):
  cost_idx = random_page_cost × log2(pages) + cpu_index_tuple_cost × selectivity × rows

Estimated speedup = cost_seq / cost_idx
```

**What we know from stats**:
- `pg_stat_user_tables.seq_scan` → how often this scan happens
- `pg_stat_user_tables.seq_tup_read` → rows scanned per seq scan
- `pg_class.relpages` → table pages on disk
- `pg_class.reltuples` → estimated row count
- `pg_stat_statements.mean_exec_time` → current query time (if available)

**Example output**:
> "Table `orders` (8.2M rows, 1.2GB) receives 23,000 seq scans/hour.
> Adding `CREATE INDEX CONCURRENTLY idx_orders_customer_id ON orders(customer_id)`
> would change scan from O(8.2M rows) to O(log₂ 8.2M ≈ 23 rows lookup).
> **Estimated speedup: ~350,000x for this query pattern.**
> **Estimated time saved: ~640 CPU-seconds/hour.**"

#### Configuration Recommendations

**Method**: Formula-based estimation from documented PostgreSQL behavior.

| Change | Estimation Method | Example |
|:---|:---|:---|
| Increase `shared_buffers` | `new_hit_ratio ≈ min(99%, current_ratio + (new_buffers - old_buffers) / db_size × 100)` | 128MB→2GB on 4GB DB: ~73%→95% hit ratio |
| Increase `work_mem` | Count queries with `temp_blks_written > 0`. Each eliminated temp file ≈ 3-10x speedup on that query. | Default 4MB→64MB: eliminates disk spill for 15 queries |
| `checkpoint_completion_target` 0.5→0.9 | Spreads checkpoint I/O over longer period. Reduces I/O spikes by ~40%. | Smoother I/O, fewer latency spikes |
| `random_page_cost` 4.0→1.1 (SSD) | Planner will prefer index scans. Count queries currently doing seq scan that have usable indexes. | May enable index usage for N queries |

#### Vacuum Recommendations

**Method**: Calculate space recovery and scan speedup.

```
Wasted space = n_dead_tup × avg_row_width
Bloat ratio = n_dead_tup / (n_live_tup + n_dead_tup)
Space recoverable = table_size × bloat_ratio

Seq scan speedup after vacuum:
  current_scan_pages = relpages
  post_vacuum_pages = relpages × (1 - bloat_ratio)
  speedup = current_scan_pages / post_vacuum_pages
```

**Example output**:
> "Table `events` has 42% dead tuple ratio (3.1M dead / 7.4M total).
> `VACUUM ANALYZE events;` would reclaim ~1.8GB of space and make
> sequential scans **1.7x faster**."

#### Connection Recommendations

**Method**: Calculate memory savings from reducing idle connections.

```
Memory per connection ≈ work_mem + temp_buffers + ~10MB overhead
Idle connections × memory_per_conn = wasted memory

If using PgBouncer with pool_size = 20 instead of max_connections = 500:
  Memory saved = (500 - 20) × memory_per_conn
  This memory becomes available for shared_buffers or OS page cache
```

---

## Accuracy & Confidence Model

Every finding includes a **confidence level**:

### Confidence Tiers

| Confidence | Label | When Applied | Accuracy |
|:---|:---|:---|:---|
| **HIGH** (90%+) | "Verified" | Threshold rules with well-documented PG best practices | Very reliable. Based on PostgreSQL documentation and community consensus over 20+ years. |
| **MEDIUM** (70-90%) | "Estimated" | Formula rules, pattern-based index suggestions | Usually correct. The recommendation is sound but the quantified impact may vary by ±2-5x. |
| **LOW** (50-70%) | "Heuristic" | Bloat estimation without `pgstattuple`, schema suggestions | Directionally correct but numbers are rough. Use as a starting point for investigation. |

### What Affects Accuracy

| Factor | Impact on Accuracy | Mitigation |
|:---|:---|:---|
| **Stats age** | If `pg_stat_reset` was recent, counters may not reflect real workload | We check `stats_reset` timestamp and warn if < 24 hours |
| **Workload variability** | A batch job at 3am may skew daily averages | Agent mode captures time-series, enabling workload pattern detection |
| **PG version** | Newer versions have more stats views | We detect version and adjust available checks |
| **Missing extensions** | No `pg_stat_statements` = no query analysis | We report what we couldn't analyze and recommend extension installation |
| **Bloat estimation** | Without `pgstattuple`, bloat is statistical guess | We clearly label as "estimated" and suggest `pgstattuple` for exact numbers |
| **RAM detection** | Via SQL we can't always know total system RAM | Tier 3 (agent) reads `/proc/meminfo`; otherwise we ask or infer from `shared_buffers` |

### Per-Category Accuracy

| Category | Confidence | Notes |
|:---|:---|:---|
| **Configuration** | 🟢 HIGH | Well-documented formulas. The "25% RAM for shared_buffers" rule is decades old. |
| **Unused Indexes** | 🟢 HIGH | Binary check: `idx_scan = 0`. Only risk: stats haven't accumulated long enough. We check stats age. |
| **Missing Indexes** | 🟡 MEDIUM | We detect the symptom (high seq_scan on large table) but the exact columns need `pg_qualstats` (Tier 3) or manual investigation. |
| **Table Dead Tuples** | 🟢 HIGH | Direct read from `pg_stat_user_tables`. Numbers are exact. |
| **Table Bloat** | 🟡 MEDIUM | Statistical estimation. Can be off by 10-30%. `pgstattuple` gives exact answer. |
| **Query Analysis** | 🟢 HIGH | `pg_stat_statements` numbers are exact. The improvement estimate after fix is MEDIUM confidence. |
| **Cache Hit Ratio** | 🟢 HIGH | Direct computation from `blks_hit / (blks_hit + blks_read)`. Exact number. |
| **Connection Analysis** | 🟢 HIGH | Direct read from `pg_stat_activity`. Current snapshot is exact. |
| **Vacuum Status** | 🟢 HIGH | Timestamps and counts are exact from `pg_stat_user_tables`. |
| **Txid Wraparound** | 🟢 HIGH | `age(datfrozenxid)` is exact. Threshold of 2B is a known PostgreSQL limit. |
| **Lock Detection** | 🟢 HIGH | `pg_locks` shows current state exactly. But it's a point-in-time snapshot — transient locks may be missed. |
| **Sequence Exhaustion** | 🟢 HIGH | `last_value / max_value` is exact math. |
| **Improvement Estimates** | 🟡 MEDIUM | Order-of-magnitude correct (e.g., "5-20x faster") but not precise. Real improvement depends on workload, hardware, and concurrent access patterns. |

### How We Validate Predictions

For each improvement estimate, we show the math:

```
┌─────────────────────────────────────────────────────┐
│ FINDING: Missing index on orders.customer_id        │
│ Severity: WARNING                                   │
│ Confidence: MEDIUM                                  │
│                                                     │
│ Current State:                                      │
│   • Table: orders (8.2M rows, 1.2GB, 163,840 pages)│
│   • Sequential scans: 23,000/hour                   │
│   • Avg rows per scan: 8,200,000 (full table)       │
│                                                     │
│ Prediction:                                         │
│   • Index scan would read: ~23 pages (btree depth)  │
│   • Speedup per query: ~7,000x                      │
│   • Total CPU time saved: ~640 sec/hour              │
│                                                     │
│ Confidence Notes:                                   │
│   • Speedup assumes single-row lookup pattern        │
│   • Actual speedup depends on selectivity             │
│   • Range queries would see less dramatic improvement│
│                                                     │
│ How to verify before applying:                      │
│   EXPLAIN (ANALYZE, BUFFERS)                        │
│   SELECT * FROM orders WHERE customer_id = 12345;   │
│                                                     │
│ Fix:                                                │
│   CREATE INDEX CONCURRENTLY idx_orders_customer_id   │
│   ON orders (customer_id);                          │
│                                                     │
│ ⚠️  Index creation on 1.2GB table takes ~30-60 sec  │
│   CONCURRENTLY avoids locking writes                │
└─────────────────────────────────────────────────────┘
```

### Comparison with Verification

We always provide the user with commands to verify our predictions independently:

| Our Prediction | Verification Command |
|:---|:---|
| "This query does a sequential scan" | `EXPLAIN (ANALYZE, BUFFERS) <query>` |
| "Cache hit ratio would improve" | Check `pg_stat_database` before/after `shared_buffers` change |
| "Table has ~40% bloat" | `CREATE EXTENSION pgstattuple; SELECT * FROM pgstattuple('tablename');` |
| "This index is unused" | `SELECT idx_scan FROM pg_stat_user_indexes WHERE indexrelname = 'idx_name';` |
| "work_mem increase will stop disk spill" | `SET work_mem = '64MB'; EXPLAIN (ANALYZE, BUFFERS) <query>;` |

---

## Agent Mode: Continuous Monitoring Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Agent Process                         │
│                                                             │
│  ┌─────────────┐     ┌──────────────┐     ┌──────────────┐ │
│  │  Scheduler   │────▶│  Collector    │────▶│  SQLite      │ │
│  │  (tick every │     │  (SQL + OS)   │     │  (snapshots) │ │
│  │   N seconds) │     └──────────────┘     └──────┬───────┘ │
│  └─────────────┘                                  │         │
│                                                   ▼         │
│  ┌─────────────┐     ┌──────────────┐     ┌──────────────┐ │
│  │  HTTP Server │◀────│  Analyzer     │◀────│  Rate Calc   │ │
│  │  (dashboard  │     │  (on-demand   │     │  (deltas /   │ │
│  │   + API)     │     │   or periodic)│     │   time)      │ │
│  └─────────────┘     └──────────────┘     └──────────────┘ │
│                                                             │
│  Snapshot interval: 30-60 seconds (configurable)            │
│  Analysis interval: 5 minutes (configurable)                │
│  Retention: 30 days (configurable)                          │
│  Memory usage: ~20-50MB                                     │
│  CPU usage: <1% (brief spike during collection)             │
└─────────────────────────────────────────────────────────────┘
```

### Rate Computation

Cumulative counters → rates using adjacent snapshots:

```go
type RateCalculator struct {
    previous *Snapshot
    current  *Snapshot
}

func (r *RateCalculator) BlocksReadPerSec() float64 {
    delta := r.current.BlksRead - r.previous.BlksRead
    timeDelta := r.current.TakenAt.Sub(r.previous.TakenAt).Seconds()
    return float64(delta) / timeDelta
}

func (r *RateCalculator) QueriesPerSec() float64 {
    delta := r.current.TotalQueries - r.previous.TotalQueries
    timeDelta := r.current.TakenAt.Sub(r.previous.TakenAt).Seconds()
    return float64(delta) / timeDelta
}
```

### Trend Detection

With historical snapshots, we can detect:
- **Degrading cache hit ratio** → growing dataset outpacing shared_buffers
- **Rising dead tuple count** → autovacuum falling behind
- **Increasing query time** → workload growth or plan regression
- **Connection count growth** → approaching max_connections

```go
type TrendAnalysis struct {
    Metric     string
    Direction  string  // "increasing", "decreasing", "stable"
    ChangeRate float64 // per hour
    Forecast   string  // "will hit critical threshold in ~3 days"
}
```
