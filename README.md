<p align="center">
  <h1 align="center">🐘 pgaioptimizer</h1>
  <p align="center">
    <strong>PostgreSQL AI Performance Analyzer</strong><br/>
    Get a DBA-quality health report in 30 seconds. No CloudWatch. No expensive monitoring tools.<br/>
    Just a connection string.
  </p>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#how-it-works">How It Works</a> •
  <a href="#deployment-modes">Deployment Modes</a> •
  <a href="#analysis-categories">Analysis</a> •
  <a href="#contributing">Contributing</a>
</p>

---

## Why?

- **90% of teams don't have a DBA.** Developers manage PostgreSQL but don't know what `random_page_cost` does or that their default `work_mem` of 4MB is causing every sort to spill to disk.
- **Companies upgrade RDS instances because "the database is slow"** when the actual fix is a missing index (free).
- **AWS Performance Insights costs hundreds/month.** Our tool gives deeper, more actionable insights from the data PostgreSQL already collects — for free.

## Features

- 🔍 **60+ diagnostic checks** across 12 performance categories
- 📊 **Weighted health score** (0-100) with letter grade (A+ through F)
- 🏥 **Prioritized recommendations** with copy-paste SQL fixes
- 🌐 **Works everywhere** — RDS, Aurora, Cloud SQL, Azure Database, self-hosted
- 🔧 **Deep self-hosted mode** — OS metrics, extension-powered analysis, log parsing
- ☁️ **Optional cloud enrichment** — AWS/GCP/Azure API integration for OS-level metrics
- 📈 **Historical trends** — agent mode stores snapshots for trend analysis
- 🤖 **AI-powered correlation** — connects the dots across categories (optional, OpenAI)
- 🚀 **Single binary** — no runtime dependencies, just download and run

## Quick Start

```bash
# Download the latest release
curl -sSL https://github.com/ujjwalgupta983/pgaioptimizer/releases/latest/download/pgaioptimizer_$(uname -s)_$(uname -m).tar.gz | tar xz

# Run a one-shot health scan
./pgaioptimizer scan \
  --host your-db.amazonaws.com \
  --port 5432 \
  --db myapp \
  --user analyst \
  --output report.html

# Open the report
open report.html
```

## How It Works

pgaioptimizer uses a **three-tier insight model** — the same tool works everywhere, with deeper insights where the environment allows:

| Tier | Environment | What You Get |
|:---|:---|:---|
| **Tier 1: SQL** | All PostgreSQL (RDS, Cloud SQL, self-hosted) | Query performance, index analysis, table health, config tuning, vacuum status, connection analysis, lock detection, sequence risks, cache ratios |
| **Tier 2: Cloud API** | Managed PG (optional AWS/GCP/Azure creds) | + CPU utilization, memory, disk IOPS, network throughput |
| **Tier 3: Agent** | Self-hosted only | + Per-query CPU/disk I/O, wait event profiling, smart index suggestions with simulation, buffer cache inspection, OS metrics, log analysis, config file audit |

### Key Insight

All of Tier 1 data comes from PostgreSQL's built-in statistics views (`pg_stat_*`). No extensions required for basic analysis. No CloudWatch. No log exports. **Zero additional cost.**

## Deployment Modes

### Scan Mode (one-shot, any PG)
```bash
pgaioptimizer scan --host db.example.com --db myapp --user admin
```
Connect, analyze, generate report, disconnect. No installation on the target server.

### Agent Mode (continuous, self-hosted)
```bash
# On the PostgreSQL server:
pgaioptimizer agent --interval 60s --listen :9090
```
Runs continuously, collects OS metrics + PG stats, stores historical snapshots, serves a web dashboard.

### Server Mode (multi-instance)
```bash
pgaioptimizer server --port 8080 --config instances.yaml
```
Central dashboard managing multiple PostgreSQL instances with scheduled scans.

## Analysis Categories

| # | Category | Key Checks |
|:---|:---|:---|
| 1 | **Configuration** | shared_buffers, work_mem, effective_cache_size, checkpoint tuning |
| 2 | **Indexes** | Unused, duplicate, overlapping, missing, bloated indexes |
| 3 | **Tables** | Dead tuples, bloat, sequential scan dominance, unanalyzed tables |
| 4 | **Queries** | Slowest queries, I/O heavy queries, temp file spills |
| 5 | **Connections** | Utilization, idle waste, idle-in-transaction, long-running |
| 6 | **Vacuum** | Autovacuum health, dead tuple accumulation, txid wraparound risk |
| 7 | **Cache** | Hit ratios, checkpoint frequency, background writer efficiency |
| 8 | **Locks** | Conflicts, blocking chains, deadlock history |
| 9 | **Sequences** | Exhaustion risk, int4 overflow, cache tuning |
| 10 | **Storage** | Database sizes, WAL rate, TOAST overhead |
| 11 | **Replication** | Lag detection, inactive slots, standby health |
| 12 | **Schema** | Missing PKs, data type choices, constraint analysis |

## Tech Stack

- **Core:** Go 1.22+
- **PG Driver:** pgx v5
- **Frontend:** Vite + React + TypeScript
- **Storage:** SQLite (embedded, pure-Go)
- **CLI:** Cobra + Viper

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License — see [LICENSE](LICENSE) for details.
