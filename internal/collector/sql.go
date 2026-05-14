// Package collector handles data collection from PostgreSQL via SQL queries.
package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// Collector manages the connection to a PostgreSQL instance and collects stats.
type Collector struct {
	pool *pgxpool.Pool
}

// New creates a new Collector with a connection pool to the target PostgreSQL.
func New(ctx context.Context, dsn string) (*Collector, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {	
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	return &Collector{pool: pool}, nil
}

// Pool returns the underlying connection pool for analyzers to use.
func (c *Collector) Pool() *pgxpool.Pool {
	return c.pool
}

// Close closes the connection pool.
func (c *Collector) Close() {
	if c.pool != nil {
		c.pool.Close()
	}
}

// ServerInfo collects basic information about the PostgreSQL server.
func (c *Collector) ServerInfo(ctx context.Context) (*models.ServerInfo, error) {
	info := &models.ServerInfo{
		ConnectionTier: "sql_only",
	}

	// Get version
	err := c.pool.QueryRow(ctx, "SELECT version()").Scan(&info.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	// Get version number
	var versionNumStr string
	err = c.pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionNumStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get version num: %w", err)
	}
	info.VersionNum, _ = strconv.Atoi(versionNumStr)

	// Get current database
	err = c.pool.QueryRow(ctx, "SELECT current_database()").Scan(&info.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Check superuser
	err = c.pool.QueryRow(ctx,
		"SELECT current_setting('is_superuser') = 'on' OR EXISTS(SELECT 1 FROM pg_roles WHERE rolname = current_user AND rolsuper)").
		Scan(&info.IsSuperuser)
	if err != nil {
		info.IsSuperuser = false // Non-critical, continue
	}

	// Get uptime
	err = c.pool.QueryRow(ctx,
		"SELECT now() - pg_postmaster_start_time()").
		Scan(&info.Uptime)
	if err != nil {
		info.Uptime = "unknown"
	}

	// Get installed extensions
	rows, err := c.pool.Query(ctx,
		"SELECT extname FROM pg_extension ORDER BY extname")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ext string
			if err := rows.Scan(&ext); err == nil {
				info.Extensions = append(info.Extensions, ext)
			}
		}
	}

	// Try to get data directory (requires superuser on some setups)
	_ = c.pool.QueryRow(ctx, "SHOW data_directory").Scan(&info.DataDirectory)

	return info, nil
}

// HasExtension checks if a specific extension is installed.
func (c *Collector) HasExtension(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := c.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)", name).
		Scan(&exists)
	return exists, err
}
