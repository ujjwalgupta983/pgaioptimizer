package engine

import (
	"strings"

	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// Correlator examines findings across multiple categories to find shared root causes.
type Correlator struct{}

// NewCorrelator creates a new correlator instance.
func NewCorrelator() *Correlator {
	return &Correlator{}
}

// Correlate takes a list of category scores and returns cross-category correlated findings.
func (c *Correlator) Correlate(scores []*models.CategoryScore) []models.Finding {
	findingsByCategory := make(map[string][]models.Finding)
	for _, score := range scores {
		findingsByCategory[score.Category] = score.Findings
	}

	var correlated []models.Finding

	// Rule 1: Undersized shared_buffers causing low cache hit ratio and seq scans
	if f := c.checkCacheMissRootCause(findingsByCategory); f != nil {
		correlated = append(correlated, *f)
	}

	// Rule 2: Low work_mem causing disk spills
	if f := c.checkWorkMemSpill(findingsByCategory); f != nil {
		correlated = append(correlated, *f)
	}

	// Rule 3: Idle in transaction blocking vacuum causing dead tuples
	if f := c.checkIdleVacuumBlock(findingsByCategory); f != nil {
		correlated = append(correlated, *f)
	}

	return correlated
}

func (c *Correlator) checkCacheMissRootCause(findingsByCategory map[string][]models.Finding) *models.Finding {
	hasLowCache := hasFindingWithSeverity(findingsByCategory["cache"], models.SeverityCritical)
	hasSmallBuffers := hasFindingWithTitle(findingsByCategory["configuration"], "shared_buffers")
	hasSeqScans := hasFindingWithTitle(findingsByCategory["tables"], "High sequential scans")

	if hasLowCache && hasSmallBuffers && hasSeqScans {
		return &models.Finding{
			Category:         "correlation",
			Title:            "Root Cause: Undersized shared_buffers causing cascade effect",
			Severity:         models.SeverityCritical,
			Description:      "Low cache hit ratio is being caused by an undersized shared_buffers setting combined with large tables undergoing sequential scans. The cache is too small to hold the tables being repeatedly scanned.",
			CurrentValue:     "Multiple correlated issues",
			RecommendedValue: "Fix underlying configuration and index issues",
			Impact:           "Dramatically slows down database performance due to constant disk reading.",
			SQLFix:           "1. Increase shared_buffers (see Configuration section).\n2. Add indexes to tables with high sequential scans (see Tables section).",
		}
	}
	return nil
}

func (c *Correlator) checkWorkMemSpill(findingsByCategory map[string][]models.Finding) *models.Finding {
	hasLowWorkMem := hasFindingWithTitle(findingsByCategory["configuration"], "work_mem")
	hasDiskSpill := hasFindingWithTitle(findingsByCategory["queries"], "Query is spilling to disk")

	if hasLowWorkMem && hasDiskSpill {
		return &models.Finding{
			Category:         "correlation",
			Title:            "Root Cause: Default work_mem causing query disk spills",
			Severity:         models.SeverityWarning,
			Description:      "Queries are writing temporary files to disk because work_mem is left at its default low value, preventing in-memory sorting/hashing.",
			CurrentValue:     "Multiple correlated issues",
			RecommendedValue: "Increase work_mem",
			Impact:           "Disk spilling slows down complex queries (ORDER BY, GROUP BY, hash joins) by 10-100x.",
			SQLFix:           "Increase work_mem setting (see Configuration section).",
		}
	}
	return nil
}

func (c *Correlator) checkIdleVacuumBlock(findingsByCategory map[string][]models.Finding) *models.Finding {
	hasIdleInTxn := hasFindingWithTitle(findingsByCategory["connections"], "Idle in transaction")
	hasDeadTuples := hasFindingWithSeverity(findingsByCategory["vacuum"], models.SeverityWarning) || hasFindingWithSeverity(findingsByCategory["vacuum"], models.SeverityCritical)

	if hasIdleInTxn && hasDeadTuples {
		return &models.Finding{
			Category:         "correlation",
			Title:            "Root Cause: Idle transactions blocking Autovacuum",
			Severity:         models.SeverityWarning,
			Description:      "High dead tuple ratios (bloat) are being caused by connections left 'idle in transaction'. These connections prevent autovacuum from cleaning up any rows modified since the transaction started.",
			CurrentValue:     "Multiple correlated issues",
			RecommendedValue: "Terminate idle connections",
			Impact:           "Severe table bloat that requires VACUUM FULL (exclusive locks) to resolve.",
			SQLFix:           "1. Set idle_in_transaction_session_timeout.\n2. Fix application code to close transactions promptly.",
		}
	}
	return nil
}

// Helpers
func hasFindingWithSeverity(findings []models.Finding, severity models.Severity) bool {
	for _, f := range findings {
		if f.Severity == severity {
			return true
		}
	}
	return false
}

func hasFindingWithTitle(findings []models.Finding, titlePrefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f.Title, titlePrefix) {
			return true
		}
	}
	return false
}
