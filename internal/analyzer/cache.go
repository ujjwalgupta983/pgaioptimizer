package analyzer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	cacheWeight = 0.10 // 10% of overall score
)

const queryCacheHitRatio = `
SELECT 
    sum(blks_hit) AS total_blks_hit,
    sum(blks_read) AS total_blks_read,
    CASE 
        WHEN sum(blks_hit) + sum(blks_read) > 0 
        THEN sum(blks_hit)::float / (sum(blks_hit) + sum(blks_read)) 
        ELSE 1.0 
    END AS cache_hit_ratio
FROM pg_stat_database
`

const queryBgwriter = `
SELECT 
    checkpoints_req,
    checkpoints_timed,
    buffers_checkpoint,
    buffers_clean,
    maxwritten_clean,
    buffers_backend,
    buffers_alloc
FROM pg_stat_bgwriter
`

type CacheAnalyzer struct{}

func (a *CacheAnalyzer) Name() string        { return "cache" }
func (a *CacheAnalyzer) Description() string { return "Buffer cache hit ratio and bgwriter efficiency" }
func (a *CacheAnalyzer) Weight() float64     { return cacheWeight }

func (a *CacheAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	a.analyzeCacheHitRatio(ctx, pool, score)
	a.analyzeBgwriter(ctx, pool, score)

	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

func (a *CacheAnalyzer) analyzeCacheHitRatio(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	var totalHit, totalRead int64
	var hitRatio float64

	err := pool.QueryRow(ctx, queryCacheHitRatio).Scan(&totalHit, &totalRead, &hitRatio)
	if err != nil {
		return
	}

	if hitRatio < 0.80 {
		score.Score -= 40
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Critical: Low Cache Hit Ratio",
			Severity:         models.SeverityCritical,
			Description:      fmt.Sprintf("Global cache hit ratio is %.1f%%. The database is frequently reading from disk instead of memory.", hitRatio*100),
			CurrentValue:     fmt.Sprintf("%.1f%%", hitRatio*100),
			RecommendedValue: "> 95%",
			Impact:           "Disk I/O is the biggest bottleneck in PostgreSQL. Low hit ratios indicate undersized shared_buffers, missing indexes, or a dataset that far exceeds available RAM.",
			SQLFix:           "1. Check shared_buffers setting.\n2. Identify missing indexes.\n3. Consider upgrading instance memory if dataset is very large.",
		})
	} else if hitRatio < 0.95 {
		score.Score -= 15
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Warning: Suboptimal Cache Hit Ratio",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("Global cache hit ratio is %.1f%%. It is generally recommended to be above 95%%.", hitRatio*100),
			CurrentValue:     fmt.Sprintf("%.1f%%", hitRatio*100),
			RecommendedValue: "> 95%",
			Impact:           "Some queries are experiencing higher latency due to disk reads.",
			SQLFix:           "Review shared_buffers and index usage.",
		})
	}
}

func (a *CacheAnalyzer) analyzeBgwriter(ctx context.Context, pool *pgxpool.Pool, score *models.CategoryScore) {
	var req, timed, bufCkpt, bufClean, maxClean, bufBackend, bufAlloc int64
	err := pool.QueryRow(ctx, queryBgwriter).Scan(&req, &timed, &bufCkpt, &bufClean, &maxClean, &bufBackend, &bufAlloc)
	if err != nil {
		return
	}

	totalCheckpoints := req + timed
	if totalCheckpoints == 0 {
		return
	}

	// High requested checkpoints vs timed checkpoints
	reqRatio := float64(req) / float64(totalCheckpoints)
	if reqRatio > 0.5 {
		score.Score -= 10
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "High frequency of requested checkpoints",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("%.1f%% of checkpoints are requested (forced by WAL limits) rather than timed.", reqRatio*100),
			CurrentValue:     fmt.Sprintf("%d requested, %d timed", req, timed),
			RecommendedValue: "< 10% requested",
			Impact:           "Frequent forced checkpoints cause massive I/O spikes and reduce overall throughput.",
			SQLFix:           "Increase max_wal_size to allow checkpoints to be spaced out by checkpoint_timeout.",
		})
	}

	// maxwritten_clean > 0 means bgwriter had to stop because it hit bgwriter_lru_maxpages
	if maxClean > 100 {
		score.Score -= 5
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "Background writer hitting max limits",
			Severity:         models.SeverityInfo,
			Description:      fmt.Sprintf("bgwriter halted %d times because it hit bgwriter_lru_maxpages limit.", maxClean),
			CurrentValue:     fmt.Sprintf("%d times", maxClean),
			RecommendedValue: "0",
			Impact:           "Forces backends (queries) to do their own buffer cleaning, slowing them down.",
			SQLFix:           "Increase bgwriter_lru_maxpages and/or bgwriter_lru_multiplier.",
		})
	}
}
