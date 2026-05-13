// Package models defines the core data structures used throughout pgaioptimizer.
package models

import "time"

// Severity represents the urgency level of a finding.
type Severity string

const (
	SeverityCritical Severity = "critical" // Immediate action required
	SeverityWarning  Severity = "warning"  // Should be addressed soon
	SeverityInfo     Severity = "info"     // Nice-to-have improvement
	SeverityOK       Severity = "ok"       // No issues found
)

// Finding represents a single diagnostic finding from an analyzer.
type Finding struct {
	Category         string   `json:"category"`
	Title            string   `json:"title"`
	Severity         Severity `json:"severity"`
	Description      string   `json:"description"`
	CurrentValue     string   `json:"current_value"`
	RecommendedValue string   `json:"recommended_value,omitempty"`
	Impact           string   `json:"impact"`
	SQLFix           string   `json:"sql_fix,omitempty"`
}

// CategoryScore holds the analysis results for one category.
type CategoryScore struct {
	Category    string    `json:"category"`
	Score       float64   `json:"score"`       // 0-100
	Weight      float64   `json:"weight"`      // Percentage weight in overall score
	Findings    []Finding `json:"findings"`
	Description string    `json:"description"` // Human-readable category description
}

// Grade converts a numeric score to a letter grade.
func Grade(score float64) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// ServerInfo contains metadata about the analyzed PostgreSQL instance.
type ServerInfo struct {
	Version          string   `json:"version"`
	VersionNum       int      `json:"version_num"`
	Host             string   `json:"host"`
	Port             int      `json:"port"`
	Database         string   `json:"database"`
	IsSuperuser      bool     `json:"is_superuser"`
	Extensions       []string `json:"extensions"`
	Uptime           string   `json:"uptime"`
	DataDirectory    string   `json:"data_directory,omitempty"`
	ServerOS         string   `json:"server_os,omitempty"`
	TotalRAMBytes    int64    `json:"total_ram_bytes,omitempty"`
	ConnectionTier   string   `json:"connection_tier"` // "sql_only", "cloud_enriched", "agent"
}

// HealthReport is the complete output of an analysis run.
type HealthReport struct {
	OverallScore float64         `json:"overall_score"`
	Grade        string          `json:"grade"`
	Categories   []CategoryScore `json:"categories"`
	ServerInfo   ServerInfo      `json:"server_info"`
	GeneratedAt  time.Time       `json:"generated_at"`
	AnalysisTier string          `json:"analysis_tier"` // "tier1", "tier2", "tier3"
	Duration     time.Duration   `json:"duration"`
}
