package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// DuckDBStore handles connecting to DuckDB and storing analysis snapshots.
type DuckDBStore struct {
	db *sql.DB
}

// NewDuckDBStore creates a new DuckDBStore and initializes the schema.
func NewDuckDBStore(dbPath string) (*DuckDBStore, error) {
	// DuckDB will create the file if it doesn't exist
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	store := &DuckDBStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init duckdb schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *DuckDBStore) Close() error {
	return s.db.Close()
}

func (s *DuckDBStore) initSchema() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS snapshots (
		id UUID DEFAULT uuid(),
		timestamp TIMESTAMP,
		host VARCHAR,
		database VARCHAR,
		overall_score DOUBLE,
		grade VARCHAR,
		report_json JSON,
		PRIMARY KEY (id)
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// SaveSnapshot saves a HealthReport as a time-series snapshot.
func (s *DuckDBStore) SaveSnapshot(report *models.HealthReport) error {
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	query := `
	INSERT INTO snapshots (timestamp, host, database, overall_score, grade, report_json)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	timestamp := report.GeneratedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	_, err = s.db.Exec(query,
		timestamp,
		report.ServerInfo.Host,
		report.ServerInfo.Database,
		report.OverallScore,
		report.Grade,
		string(reportJSON),
	)

	return err
}
