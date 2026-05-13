# Architecture

## System Overview

```mermaid
graph TB
    subgraph "Target Environments"
        RDS["☁️ AWS RDS / Aurora"]
        GCS["☁️ GCP Cloud SQL"]
        AZ["☁️ Azure Database"]
        SH["🖥️ Self-Hosted PG"]
    end

    subgraph "pgaioptimizer"
        subgraph "Collection Layer"
            SQL["Tier 1: SQL Collector<br/>pg_stat_* views<br/>(works everywhere)"]
            CLOUD["Tier 2: Cloud Enrichment<br/>CloudWatch / GCP Mon / Azure Mon<br/>(managed PG only)"]
            AGENT["Tier 3: OS Agent<br/>/proc/* + PG extensions<br/>(self-hosted only)"]
        end

        subgraph "Storage Layer"
            MEM["In-Memory Ring Buffer<br/>(last 2 hours, real-time)"]
            DUCK["DuckDB Columnar Store<br/>(2h–30d, compressed)"]
        end

        subgraph "Analysis Layer"
            RULES["Rule Engine<br/>Threshold · Formula<br/>Pattern · Correlation"]
            SCORE["Scoring Engine<br/>Weighted 0–100"]
            PRED["Prediction Engine<br/>Impact Estimation"]
            CORR["Cross-Category<br/>Correlator"]
        end

        subgraph "Output Layer"
            API["REST API"]
            HTML["HTML Report"]
            JSON["JSON Export"]
            DASH["Web Dashboard<br/>React + Recharts"]
        end
    end

    RDS --> SQL
    GCS --> SQL
    AZ --> SQL
    SH --> SQL
    RDS -.->|optional| CLOUD
    GCS -.->|optional| CLOUD
    AZ -.->|optional| CLOUD
    SH --> AGENT

    SQL --> MEM
    CLOUD --> MEM
    AGENT --> MEM
    MEM --> DUCK
    MEM --> RULES
    DUCK --> RULES
    RULES --> SCORE
    RULES --> PRED
    RULES --> CORR
    SCORE --> API
    PRED --> API
    CORR --> API
    API --> HTML
    API --> JSON
    API --> DASH
```

---

## Data Collection Pipeline

```mermaid
sequenceDiagram
    participant CLI as CLI / Scheduler
    participant COL as Collector
    participant PG as Target PostgreSQL
    participant OS as OS (/proc/*)
    participant CW as Cloud API
    participant BUF as Ring Buffer
    participant DB as DuckDB

    CLI->>COL: trigger collection
    
    par Tier 1: SQL Stats
        COL->>PG: SELECT * FROM pg_stat_database
        COL->>PG: SELECT * FROM pg_stat_user_tables
        COL->>PG: SELECT * FROM pg_stat_user_indexes
        COL->>PG: SELECT * FROM pg_stat_statements
        COL->>PG: SELECT * FROM pg_stat_activity
        COL->>PG: SELECT * FROM pg_stat_bgwriter
        COL->>PG: SELECT * FROM pg_stat_wal (PG14+)
        COL->>PG: SELECT * FROM pg_stat_io (PG16+)
        COL->>PG: SELECT * FROM pg_settings
        COL->>PG: SELECT * FROM pg_locks
        COL->>PG: SELECT * FROM pg_sequences
        COL->>PG: SELECT * FROM pg_stat_replication
        PG-->>COL: raw metric rows
    and Tier 2: Cloud Metrics (if configured)
        COL->>CW: GetMetricData(CPUUtilization, FreeableMemory, ...)
        CW-->>COL: OS-level metrics
    and Tier 3: OS Metrics (agent mode only)
        COL->>OS: read /proc/stat (CPU)
        COL->>OS: read /proc/meminfo (memory)
        COL->>OS: read /proc/diskstats (disk I/O)
        COL->>OS: read /proc/{pg_pid}/* (per-backend)
        OS-->>COL: system metrics
    end

    COL->>BUF: store snapshot (in-memory)
    COL->>DB: persist snapshot (async, columnar)
    
    Note over BUF,DB: Ring buffer holds ~7,200 snapshots (2h @ 1/sec)<br/>DuckDB holds 30 days compressed
```

### Collection Timing

| Metric Group | Queries | Avg Latency | Frequency |
|:---|:---|:---|:---|
| Database stats | 3 queries | ~5ms | Every snapshot |
| Table stats | 2 queries | ~10-50ms (depends on table count) | Every snapshot |
| Index stats | 2 queries | ~10-50ms | Every snapshot |
| Query stats | 1 query | ~5-20ms | Every snapshot |
| Connection stats | 1 query | ~3ms | Every snapshot |
| Lock stats | 1 query | ~3ms | Every snapshot |
| Settings | 1 query | ~5ms | Every 5 minutes (rarely changes) |
| Schema info | 3-5 queries | ~20-100ms | Every 15 minutes |
| OS metrics | filesystem reads | ~1ms | Every snapshot (Tier 3) |
| **Total** | **~15-20 queries** | **~50-250ms** | **Every 30-60 seconds** |

---

## Storage Architecture

### Why DuckDB over SQLite

Our query patterns are **analytical** — aggregations over time ranges, rate computations, trend detection. DuckDB is purpose-built for this.

| Query Pattern | SQLite (row-store) | DuckDB (columnar) |
|:---|:---|:---|
| "Avg cache hit ratio over 7 days" | Scans all rows, all columns | Reads only 1 column, vectorized |
| "Top 10 queries by time delta in 24h" | Full join + sort | Columnar join, 10-50x faster |
| "Rate of 15 metrics over 30 days" | ~3 seconds | ~30ms |
| "Downsample to 1h buckets" | Manual GROUP BY, slow | Native window functions, fast |
| Compression ratio | ~1x (no compression) | ~5-10x (columnar + zstd) |
| Storage for 30 days | ~200-500MB | ~30-80MB |

### Storage Tiers

```mermaid
graph LR
    subgraph "Hot — In-Memory Ring Buffer"
        direction TB
        H1["Last 2 hours of snapshots"]
        H2["~7,200 data points"]
        H3["Instant access, no disk I/O"]
        H4["Used for: real-time dashboard"]
    end

    subgraph "Warm — DuckDB (recent)"
        direction TB
        W1["2 hours → 7 days"]
        W2["1-minute granularity"]
        W3["Columnar + zstd compressed"]
        W4["Used for: trend analysis, alerts"]
    end

    subgraph "Cold — DuckDB (historical)"
        direction TB
        C1["7 → 30 days"]
        C2["Downsampled to 5-min buckets"]
        C3["Heavily compressed"]
        C4["Used for: long-term trends, reports"]
    end

    H1 -->|"flush every 2h"| W1
    W1 -->|"downsample at 7d"| C1
    C1 -->|"prune at 30d"| DEL["🗑️ Auto-delete"]
```

### DuckDB Schema

```sql
-- Snapshot metadata
CREATE TABLE snapshots (
    id            UINTEGER PRIMARY KEY,
    taken_at      TIMESTAMP NOT NULL,
    duration_ms   USMALLINT,
    pg_version    UINTEGER,
    tier          UTINYINT   -- 1=SQL, 2=cloud, 3=agent
);

-- Database-level metrics (one row per snapshot)
CREATE TABLE m_database (
    snapshot_id      UINTEGER NOT NULL,
    taken_at         TIMESTAMP NOT NULL,  -- denormalized for partition pruning
    xact_commit      UBIGINT,
    xact_rollback    UBIGINT,
    blks_read        UBIGINT,
    blks_hit         UBIGINT,
    tup_returned     UBIGINT,
    tup_fetched      UBIGINT,
    tup_inserted     UBIGINT,
    tup_updated      UBIGINT,
    tup_deleted      UBIGINT,
    deadlocks        UBIGINT,
    temp_files       UBIGINT,
    temp_bytes       UBIGINT,
    blk_read_time    DOUBLE,
    blk_write_time   DOUBLE
);

-- Table-level metrics (one row per table per snapshot)
CREATE TABLE m_tables (
    snapshot_id     UINTEGER NOT NULL,
    taken_at        TIMESTAMP NOT NULL,
    schema_name     VARCHAR,
    table_name      VARCHAR,
    seq_scan        UBIGINT,
    seq_tup_read    UBIGINT,
    idx_scan        UBIGINT,
    idx_tup_fetch   UBIGINT,
    n_tup_ins       UBIGINT,
    n_tup_upd       UBIGINT,
    n_tup_del       UBIGINT,
    n_live_tup      UBIGINT,
    n_dead_tup      UBIGINT,
    last_vacuum     TIMESTAMP,
    last_autovacuum TIMESTAMP,
    last_analyze    TIMESTAMP,
    table_bytes     UBIGINT,
    toast_bytes     UBIGINT,
    index_bytes     UBIGINT
);

-- Query metrics from pg_stat_statements (top N per snapshot)
CREATE TABLE m_queries (
    snapshot_id       UINTEGER NOT NULL,
    taken_at          TIMESTAMP NOT NULL,
    queryid           BIGINT,
    query_text        VARCHAR,
    calls             UBIGINT,
    total_exec_time   DOUBLE,
    mean_exec_time    DOUBLE,
    stddev_exec_time  DOUBLE,
    rows              UBIGINT,
    shared_blks_hit   UBIGINT,
    shared_blks_read  UBIGINT,
    temp_blks_read    UBIGINT,
    temp_blks_written UBIGINT,
    blk_read_time     DOUBLE,
    blk_write_time    DOUBLE
);

-- Connection state summary (one row per snapshot)
CREATE TABLE m_connections (
    snapshot_id       UINTEGER NOT NULL,
    taken_at          TIMESTAMP NOT NULL,
    total             USMALLINT,
    active            USMALLINT,
    idle              USMALLINT,
    idle_in_txn       USMALLINT,
    waiting           USMALLINT,
    max_connections   USMALLINT,
    longest_query_sec UINTEGER,
    longest_txn_sec   UINTEGER
);

-- OS metrics (Tier 3 agent only)
CREATE TABLE m_os (
    snapshot_id       UINTEGER NOT NULL,
    taken_at          TIMESTAMP NOT NULL,
    cpu_user_pct      FLOAT,
    cpu_system_pct    FLOAT,
    cpu_iowait_pct    FLOAT,
    cpu_idle_pct      FLOAT,
    mem_total_bytes   UBIGINT,
    mem_available_bytes UBIGINT,
    mem_cached_bytes  UBIGINT,
    mem_buffers_bytes UBIGINT,
    swap_used_bytes   UBIGINT,
    disk_read_ops     UBIGINT,
    disk_write_ops    UBIGINT,
    disk_read_bytes   UBIGINT,
    disk_write_bytes  UBIGINT,
    disk_io_time_ms   UBIGINT
);

-- Downsampled 5-minute rollups (auto-generated)
CREATE TABLE m_database_5m (
    bucket           TIMESTAMP NOT NULL,  -- floor to 5 min
    xact_commit_rate DOUBLE,   -- per second
    blks_read_rate   DOUBLE,
    blks_hit_rate    DOUBLE,
    cache_hit_ratio  DOUBLE,
    tup_fetched_rate DOUBLE,
    deadlocks_delta  UBIGINT,
    temp_files_delta UBIGINT
);
```

### Rate Computation

All `pg_stat_*` counters are cumulative. We compute rates from adjacent snapshots:

```mermaid
graph LR
    S1["Snapshot T₁<br/>blks_read = 1,000,000"]
    S2["Snapshot T₂<br/>blks_read = 1,005,000<br/>Δt = 60 seconds"]
    RATE["Rate = (1,005,000 − 1,000,000) / 60<br/>= 83.3 blocks/sec"]
    
    S1 --> RATE
    S2 --> RATE
```

```go
// Rate computation handles counter resets (e.g., pg_stat_reset)
func computeRate(prev, curr uint64, deltaSeconds float64) float64 {
    if curr < prev {
        return 0 // counter was reset, skip this interval
    }
    return float64(curr-prev) / deltaSeconds
}
```

---

## Analysis Engine

### Rule Processing Pipeline

```mermaid
graph TB
    subgraph "Input"
        SNAP["Current Snapshot<br/>+ Historical Data"]
    end

    subgraph "Rule Engine (per category)"
        T["① Threshold Rules<br/>Single metric vs known range<br/>Confidence: HIGH"]
        F["② Formula Rules<br/>Derived metric vs computed target<br/>Confidence: HIGH"]
        P["③ Pattern Rules<br/>Multi-condition detection<br/>Confidence: MEDIUM"]
        C["④ Correlation Rules<br/>Cross-category root cause<br/>Confidence: MEDIUM"]
    end

    subgraph "Scoring"
        SC["Category Score<br/>Start: 100<br/>CRIT: −20 to −40<br/>WARN: −5 to −20<br/>INFO: −1 to −5"]
    end

    subgraph "Prediction"
        EST["Impact Estimator<br/>Uses PG cost model<br/>+ workload stats"]
    end

    subgraph "Output"
        FIND["Findings[]<br/>severity + description<br/>+ SQL fix + confidence<br/>+ estimated impact"]
    end

    SNAP --> T --> SC
    SNAP --> F --> SC
    SNAP --> P --> SC
    SNAP --> C --> SC
    SC --> FIND
    T --> EST --> FIND
    F --> EST --> FIND
    P --> EST --> FIND
```

### Rule Type Details

#### ① Threshold Rules — compare metric to known range

```
cache_hit_ratio:
  ≥99%  → OK       (score: −0)
  ≥95%  → OK       (score: −5)
  ≥90%  → INFO     (score: −15)
  ≥80%  → WARNING  (score: −30)
  <80%  → CRITICAL (score: −50)
  confidence: HIGH (direct measurement)

dead_tuple_ratio (per table):
  <5%   → OK
  <10%  → INFO
  <20%  → WARNING  ("VACUUM ANALYZE {table};")
  ≥20%  → CRITICAL ("VACUUM FULL {table}; -- requires exclusive lock")
  confidence: HIGH (exact count from pg_stat_user_tables)

connection_utilization:
  <50%  → OK
  <70%  → INFO
  <85%  → WARNING  ("Consider PgBouncer")
  ≥85%  → CRITICAL ("Risk of connection exhaustion")
  confidence: HIGH (exact from pg_stat_activity vs max_connections)
```

#### ② Formula Rules — compute target from system params

```
shared_buffers:
  recommended = total_ram × 0.25
  IF actual < recommended × 0.6 → WARNING
  IF actual > recommended × 1.6 → INFO (diminishing returns)
  sql_fix: "ALTER SYSTEM SET shared_buffers = '{recommended}';"
  confidence: HIGH (industry standard for 20+ years)

work_mem:
  recommended = (total_ram − shared_buffers) / (3 × max_connections)
  IF actual == 4MB (default)    → WARNING
  IF actual < recommended × 0.5 → WARNING
  sql_fix: "ALTER SYSTEM SET work_mem = '{recommended}';"
  confidence: HIGH (formula from PG docs)

effective_cache_size:
  recommended = total_ram × 0.625  -- midpoint of 50-75%
  IF actual < total_ram × 0.40     → WARNING
  confidence: HIGH (planner hint, no memory allocated)
```

#### ③ Pattern Rules — multi-condition detection

```
missing_index:
  FOR EACH table WHERE:
    seq_scan > 1000
    AND n_live_tup > 100,000
    AND seq_scan > (idx_scan × 10)
  → WARNING
  Tier 3 enhancement: use pg_qualstats to identify which
    columns are filtered, suggest specific index definition
  confidence: MEDIUM (detects symptom, column choice needs verification)

unused_index:
  FOR EACH index WHERE:
    idx_scan = 0
    AND NOT indisunique
    AND NOT indisprimary
    AND stats_age > 7 days
  → INFO
  sql_fix: "DROP INDEX CONCURRENTLY {index_name};"
  confidence: HIGH (binary: used or not, with stats age check)

duplicate_index:
  FOR EACH pair of indexes on same table WHERE:
    index_columns_A == index_columns_B
    OR index_columns_A is prefix of index_columns_B
  → WARNING
  confidence: HIGH (structural comparison)
```

#### ④ Correlation Rules — cross-category root cause analysis

```mermaid
graph LR
    subgraph "Findings from Individual Categories"
        F1["Cache: hit ratio 73%"]
        F2["Config: shared_buffers 128MB<br/>on 16GB server"]
        F3["Tables: 3 tables with<br/>high seq_scan count"]
        F4["Queries: top queries<br/>have high blks_read"]
    end

    CORR["Correlator"]
    
    subgraph "Correlated Finding"
        CF["ROOT CAUSE: Undersized shared_buffers<br/>+ missing indexes cause cascade:<br/><br/>① shared_buffers is 128MB (0.8% of RAM)<br/>② Large tables can't fit in cache<br/>③ Queries read from disk on every call<br/>④ Cache hit ratio drops to 73%<br/><br/>Fix both to resolve:<br/>1. ALTER SYSTEM SET shared_buffers = '4GB'<br/>2. CREATE INDEX on hot tables<br/><br/>Estimated impact: cache ratio 73% → 97%"]
    end

    F1 --> CORR
    F2 --> CORR
    F3 --> CORR
    F4 --> CORR
    CORR --> CF
```

Correlation rules connect findings that share a root cause:

| Pattern | Categories Involved | Root Cause |
|:---|:---|:---|
| Low cache + small buffers + seq scans | Cache, Config, Tables | Undersized memory + missing indexes |
| Temp file spill + default work_mem | Queries, Config | work_mem needs tuning |
| High dead tuples + old vacuum + bloat | Vacuum, Tables, Storage | Autovacuum misconfigured |
| Connection exhaustion + idle conns | Connections, Config | Need connection pooler |
| Replica lag + high WAL rate + many writes | Replication, Storage, Tables | Write-heavy workload outpacing replica |

---

## Prediction & Impact Estimation

### Index Impact Model

Uses PostgreSQL's cost model constants to estimate speedup:

```mermaid
graph TB
    subgraph "Current State (Sequential Scan)"
        CS1["Table: orders"]
        CS2["Rows: 8.2M"]
        CS3["Pages: 163,840"]
        CS4["Seq scans/hour: 23,000"]
        CS5["Cost = seq_page_cost × pages<br/>= 1.0 × 163,840 = 163,840"]
    end

    subgraph "Predicted State (Index Scan)"
        PS1["Btree depth: log₂(8.2M) ≈ 23"]
        PS2["Pages to read: ~3-5"]
        PS3["Cost = random_page_cost × depth<br/>= 1.1 × 23 ≈ 25"]
    end

    subgraph "Impact"
        IMP["Speedup: 163,840 / 25 ≈ 6,500x<br/>Time saved: ~640 CPU-sec/hour<br/>Confidence: MEDIUM<br/>(assumes point lookup selectivity)"]
    end

    CS5 --> IMP
    PS3 --> IMP
```

### Configuration Impact Model

| Parameter Change | Estimation Formula | Confidence |
|:---|:---|:---|
| `shared_buffers` ↑ | `Δ_hit_ratio ≈ min(99%, current + (new_size − old_size) / db_size × 100)` | MEDIUM |
| `work_mem` ↑ | Count queries with `temp_blks > 0`. Each: ~3-10x speedup. | MEDIUM |
| `checkpoint_completion_target` → 0.9 | Spreads I/O, reduces spikes ~40% | HIGH |
| `random_page_cost` → 1.1 (SSD) | Planner prefers indexes. Count seq scans with usable indexes. | MEDIUM |
| `max_connections` ↓ + PgBouncer | Saves `N × ~10MB` per idle connection | HIGH |

### Vacuum Impact Model

```
Space recoverable = table_size × (n_dead_tup / (n_live_tup + n_dead_tup))
Scan speedup = 1 / (1 − dead_ratio)

Example: 42% dead tuples on 4.3GB table
  Space savings: 4.3GB × 0.42 = 1.8GB
  Scan speedup: 1 / 0.58 = 1.7x
  Confidence: HIGH (dead tuple count is exact)
```

---

## Accuracy & Confidence Model

Every finding carries a confidence level indicating how reliable the measurement and prediction are.

```mermaid
graph LR
    subgraph "🟢 HIGH Confidence (90%+)"
        H1["Cache hit ratio<br/>(exact computation)"]
        H2["Unused indexes<br/>(binary: 0 scans)"]
        H3["Dead tuple count<br/>(exact from stats)"]
        H4["Connection count<br/>(exact snapshot)"]
        H5["Txid wraparound risk<br/>(exact counter)"]
        H6["Sequence exhaustion<br/>(exact math)"]
        H7["Config vs formula<br/>(documented rules)"]
    end

    subgraph "🟡 MEDIUM Confidence (70-90%)"
        M1["Missing index suggestion<br/>(pattern-based)"]
        M2["Impact estimation<br/>(cost model)"]
        M3["Bloat estimation<br/>(statistical)"]
        M4["Config impact prediction<br/>(formula-based)"]
        M5["Workload classification<br/>(heuristic)"]
    end

    subgraph "🔴 LOW Confidence (50-70%)"
        L1["Schema design suggestions<br/>(subjective)"]
        L2["Partitioning suggestions<br/>(workload-dependent)"]
        L3["Long-term trend forecast<br/>(extrapolation)"]
    end
```

### Validation Commands

Every prediction includes a command the user can run to verify independently:

| Prediction | Verification |
|:---|:---|
| "Query does sequential scan" | `EXPLAIN (ANALYZE, BUFFERS) <query>` |
| "Index would speed up query" | `SET enable_seqscan = off; EXPLAIN ANALYZE <query>;` |
| "Table has ~40% bloat" | `CREATE EXTENSION pgstattuple; SELECT * FROM pgstattuple('table');` |
| "work_mem increase stops disk spill" | `SET work_mem='64MB'; EXPLAIN (ANALYZE, BUFFERS) <query>;` |
| "shared_buffers increase helps" | Compare `pg_stat_database.blks_hit` ratio before/after |

---

## Deployment Architecture

### Scan Mode

```mermaid
graph LR
    USER["User workstation"]
    PG["Target PostgreSQL<br/>(RDS / self-hosted)"]
    RPT["HTML Report"]

    USER -->|"pgaioptimizer scan --host ..."| PG
    PG -->|"SQL results (2-5 sec)"| USER
    USER -->|"generate"| RPT

    style USER fill:#1e293b,stroke:#3b82f6,color:#fff
    style PG fill:#1e293b,stroke:#10b981,color:#fff
    style RPT fill:#1e293b,stroke:#f59e0b,color:#fff
```

Stateless. Connect → collect → analyze → report → disconnect. No storage.

### Agent Mode

```mermaid
graph TB
    subgraph "PostgreSQL Server"
        PG["PostgreSQL"]
        AGENT["pgaioptimizer agent<br/>(Go binary, ~20MB RAM)"]
        DUCK["DuckDB file<br/>(30-80MB on disk)"]
        PROC["/proc/*<br/>OS metrics"]
        CONF["postgresql.conf"]
        LOGS["pg_log/*.log"]
    end

    subgraph "User Browser"
        DASH["Dashboard :9090"]
    end

    AGENT -->|"SQL every 60s"| PG
    AGENT -->|"read"| PROC
    AGENT -->|"parse"| CONF
    AGENT -->|"parse"| LOGS
    AGENT -->|"write snapshots"| DUCK
    DASH -->|"HTTP API"| AGENT

    style PG fill:#1e293b,stroke:#10b981,color:#fff
    style AGENT fill:#1e293b,stroke:#3b82f6,color:#fff
    style DUCK fill:#1e293b,stroke:#f59e0b,color:#fff
    style DASH fill:#1e293b,stroke:#8b5cf6,color:#fff
```

Runs on PG host. Full Tier 3 access. Serves dashboard locally.

### Server Mode (multi-instance)

```mermaid
graph TB
    subgraph "Central Server"
        SRV["pgaioptimizer server :8080"]
        DB["DuckDB<br/>(all instances)"]
        WEB["React Dashboard"]
    end

    subgraph "Instance 1: Production RDS"
        PG1["PostgreSQL"]
    end

    subgraph "Instance 2: Staging (self-hosted)"
        PG2["PostgreSQL"]
        AG2["Agent :9090"]
    end

    subgraph "Instance 3: Analytics RDS"
        PG3["PostgreSQL"]
    end

    SRV -->|"scan every 5m"| PG1
    SRV -->|"pull from agent API"| AG2
    AG2 -->|"local SQL"| PG2
    SRV -->|"scan every 5m"| PG3
    SRV --> DB
    WEB --> SRV

    style SRV fill:#1e293b,stroke:#3b82f6,color:#fff
    style DB fill:#1e293b,stroke:#f59e0b,color:#fff
    style PG1 fill:#1e293b,stroke:#10b981,color:#fff
    style PG2 fill:#1e293b,stroke:#10b981,color:#fff
    style PG3 fill:#1e293b,stroke:#10b981,color:#fff
    style AG2 fill:#1e293b,stroke:#8b5cf6,color:#fff
```

---

## Security Model

```mermaid
graph LR
    subgraph "Minimal Permissions Required"
        R1["pg_monitor role<br/>(PG10+, read-only stats)"]
        R2["pg_read_all_stats<br/>(alternative)"]
    end

    subgraph "What We NEVER Do"
        N1["❌ No writes to target DB"]
        N2["❌ No DDL execution"]
        N3["❌ No temp tables"]
        N4["❌ No function creation"]
        N5["❌ No data access (SELECT on user tables)"]
    end
```

**Recommended connection setup:**
```sql
-- Create a dedicated read-only role
CREATE ROLE pgaioptimizer LOGIN PASSWORD '...';
GRANT pg_monitor TO pgaioptimizer;
-- For pg_stat_statements access:
GRANT EXECUTE ON FUNCTION pg_stat_statements_reset TO pgaioptimizer; -- optional
```

All credentials are stored in memory only during the session (scan mode) or in an encrypted config file (agent/server mode). Never logged, never transmitted.
