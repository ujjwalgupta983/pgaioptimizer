package analyzer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

const (
	configurationWeight = 0.15 // 15% of overall score
)

const querySettings = `
SELECT name, setting, unit, boot_val, reset_val
FROM pg_settings
WHERE name IN (
	'shared_buffers',
	'effective_cache_size',
	'work_mem',
	'maintenance_work_mem',
	'max_connections',
	'random_page_cost',
	'effective_io_concurrency',
	'checkpoint_completion_target',
	'wal_buffers',
	'log_min_duration_statement'
)
`

type settingRow struct {
	Name     string
	Setting  string
	Unit     *string
	BootVal  string
	ResetVal string
}

type ConfigurationAnalyzer struct{}

func (a *ConfigurationAnalyzer) Name() string { return "configuration" }
func (a *ConfigurationAnalyzer) Description() string {
	return "Database configuration and tuning parameters"
}
func (a *ConfigurationAnalyzer) Weight() float64 { return configurationWeight }

func (a *ConfigurationAnalyzer) Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error) {
	score := &models.CategoryScore{
		Category:    a.Name(),
		Score:       100,
		Weight:      a.Weight(),
		Description: a.Description(),
	}

	rows, err := pool.Query(ctx, querySettings)
	if err != nil {
		return nil, fmt.Errorf("configuration analysis failed: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]settingRow)
	for rows.Next() {
		var r settingRow
		if err := rows.Scan(&r.Name, &r.Setting, &r.Unit, &r.BootVal, &r.ResetVal); err != nil {
			continue
		}
		settings[r.Name] = r
	}

	a.analyzeSharedBuffers(settings, score, info)
	a.analyzeWorkMem(settings, score, info)
	a.analyzeConnections(settings, score, info)
	a.analyzeCheckpointTarget(settings, score, info)

	// Floor at 0
	if score.Score < 0 {
		score.Score = 0
	}
	return score, nil
}

// Convert pg_settings value with unit to bytes (e.g., '16MB', '8kB')
func parseSettingBytes(val string, unit *string) int64 {
	v, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}
	if unit == nil {
		return v
	}

	multiplier := int64(1)
	switch *unit {
	case "8kB":
		multiplier = 8192
	case "kB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	}
	return v * multiplier
}

func (a *ConfigurationAnalyzer) analyzeSharedBuffers(settings map[string]settingRow, score *models.CategoryScore, info *models.ServerInfo) {
	sb, ok := settings["shared_buffers"]
	if !ok {
		return
	}

	bytes := parseSettingBytes(sb.Setting, sb.Unit)
	mb := bytes / (1024 * 1024)

	// If we don't know total RAM, we can only warn if it's suspiciously low (e.g., default 128MB)
	if mb <= 128 {
		score.Score -= 20
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "shared_buffers is at default or very low",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("Current shared_buffers is %dMB. This is the memory PostgreSQL uses to cache data.", mb),
			CurrentValue:     fmt.Sprintf("%d MB", mb),
			RecommendedValue: "25% of total system RAM",
			Impact:           "Low cache size causes frequent disk reads, dramatically slowing down queries.",
			SQLFix:           "ALTER SYSTEM SET shared_buffers = '...GB';\n-- Requires PostgreSQL restart",
		})
	}
}

func (a *ConfigurationAnalyzer) analyzeWorkMem(settings map[string]settingRow, score *models.CategoryScore, info *models.ServerInfo) {
	wm, ok := settings["work_mem"]
	if !ok {
		return
	}

	bytes := parseSettingBytes(wm.Setting, wm.Unit)
	mb := bytes / (1024 * 1024)

	if mb <= 4 {
		score.Score -= 15
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "work_mem is at default",
			Severity:         models.SeverityWarning,
			Description:      "work_mem is 4MB. This determines how much memory can be used for sorts and hash tables before writing to disk.",
			CurrentValue:     "4 MB",
			RecommendedValue: "16MB - 64MB (depending on connection count)",
			Impact:           "Queries with ORDER BY, GROUP BY, or Hash Joins will write temporary files to disk, which is 10-100x slower.",
			SQLFix:           "ALTER SYSTEM SET work_mem = '32MB';\nSELECT pg_reload_conf();",
		})
	}
}

func (a *ConfigurationAnalyzer) analyzeConnections(settings map[string]settingRow, score *models.CategoryScore, info *models.ServerInfo) {
	mc, ok := settings["max_connections"]
	if !ok {
		return
	}

	conns, _ := strconv.Atoi(mc.Setting)
	if conns > 200 {
		score.Score -= 5
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "High max_connections without a pooler",
			Severity:         models.SeverityInfo,
			Description:      fmt.Sprintf("max_connections is set to %d. PostgreSQL processes consume significant memory. High connection counts can lead to OOM or high context switching overhead.", conns),
			CurrentValue:     strconv.Itoa(conns),
			RecommendedValue: "Use PgBouncer and reduce max_connections to 50-100",
			Impact:           "Saves memory and CPU overhead.",
			SQLFix:           "ALTER SYSTEM SET max_connections = '100';\n-- Requires PostgreSQL restart",
		})
	}
}

func (a *ConfigurationAnalyzer) analyzeCheckpointTarget(settings map[string]settingRow, score *models.CategoryScore, info *models.ServerInfo) {
	ct, ok := settings["checkpoint_completion_target"]
	if !ok {
		return
	}

	target, _ := strconv.ParseFloat(ct.Setting, 64)
	if target < 0.9 {
		score.Score -= 5
		score.Findings = append(score.Findings, models.Finding{
			Category:         a.Name(),
			Title:            "checkpoint_completion_target is low",
			Severity:         models.SeverityWarning,
			Description:      fmt.Sprintf("Current value is %.2f. This controls how spread out checkpoint disk writes are.", target),
			CurrentValue:     fmt.Sprintf("%.2f", target),
			RecommendedValue: "0.9",
			Impact:           "Lower values cause intensive I/O spikes during checkpoints, slowing down all queries.",
			SQLFix:           "ALTER SYSTEM SET checkpoint_completion_target = '0.9';\nSELECT pg_reload_conf();",
		})
	}
}
