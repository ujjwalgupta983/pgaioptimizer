package report

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// GenerateJSON writes the HealthReport to a JSON file.
func GenerateJSON(report *models.HealthReport, filepath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}
