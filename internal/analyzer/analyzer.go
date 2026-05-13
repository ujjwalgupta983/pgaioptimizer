// Package analyzer defines the interface all analysis modules must implement.
package analyzer

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// Analyzer is the interface that all analysis modules implement.
// Each analyzer follows the pattern: collect → analyze → score → recommend.
type Analyzer interface {
	// Name returns the category name (e.g., "configuration", "indexes").
	Name() string

	// Description returns a human-readable description of what this analyzer checks.
	Description() string

	// Weight returns the percentage weight of this category in the overall health score.
	Weight() float64

	// Analyze connects to the database, collects stats, runs analysis, and returns findings.
	Analyze(ctx context.Context, pool *pgxpool.Pool, info *models.ServerInfo) (*models.CategoryScore, error)
}

// Registry holds all registered analyzers.
type Registry struct {
	analyzers []Analyzer
}

// NewRegistry creates a new analyzer registry with all built-in analyzers.
func NewRegistry() *Registry {
	return &Registry{
		analyzers: []Analyzer{
			// Phase 2: Core analyzers
			&ConfigurationAnalyzer{},
			&IndexAnalyzer{},
			&TableAnalyzer{},
			&QueryAnalyzer{},
			&ConnectionAnalyzer{},
			&VacuumAnalyzer{},

			// Phase 3: Extended analyzers
			// &CacheAnalyzer{},
			// &LockAnalyzer{},
			// &SequenceAnalyzer{},
			// &StorageAnalyzer{},
			// &ReplicationAnalyzer{},
			// &SchemaAnalyzer{},
		},
	}
}

// All returns all registered analyzers.
func (r *Registry) All() []Analyzer {
	return r.analyzers
}

// Register adds an analyzer to the registry.
func (r *Registry) Register(a Analyzer) {
	r.analyzers = append(r.analyzers, a)
}
