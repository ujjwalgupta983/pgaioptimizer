package engine

import (
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// ScoringEngine computes the overall database health score.
type ScoringEngine struct{}

// NewScoringEngine creates a new ScoringEngine.
func NewScoringEngine() *ScoringEngine {
	return &ScoringEngine{}
}

// ComputeOverall computes the weighted overall score from all category scores.
func (e *ScoringEngine) ComputeOverall(scores []*models.CategoryScore) float64 {
	var overallScore float64
	var totalWeight float64

	for _, score := range scores {
		overallScore += score.Score * score.Weight
		totalWeight += score.Weight
	}

	// If total weight is not exactly 1.0 (e.g. some analyzers skipped), normalize it
	if totalWeight > 0 && totalWeight != 1.0 {
		overallScore = overallScore / totalWeight
	}

	return overallScore
}

// ComputeGrade assigns a letter grade based on the score.
func (e *ScoringEngine) ComputeGrade(score float64) string {
	if score >= 90 {
		return "A"
	}
	if score >= 80 {
		return "B"
	}
	if score >= 70 {
		return "C"
	}
	if score >= 60 {
		return "D"
	}
	return "F"
}

// GenerateReport compiles all scores, findings, and correlations into the final report.
func (e *ScoringEngine) GenerateReport(info *models.ServerInfo, scores []*models.CategoryScore, correlations []models.Finding) *models.HealthReport {
	overallScore := e.ComputeOverall(scores)

	return &models.HealthReport{
		ServerInfo:   *info,
		Categories:   scores,
		Correlations: correlations,
		OverallScore: overallScore,
		Grade:        e.ComputeGrade(overallScore),
	}
}
