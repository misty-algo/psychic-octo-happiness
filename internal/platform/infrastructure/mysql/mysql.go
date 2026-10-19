// Package mysql owns the process-level MySQL resource.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"os"
	"strconv"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/NSObjects/echo-admin/internal/platform/configs"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/resources"
)

// Resource wraps an optional GORM MySQL connection and its lifecycle.
type Resource struct {
	enabled bool
	db      *gorm.DB
}

// Open creates the configured MySQL resource. cfg must come from an
// already-normalized configs.Load result; disabled resources do not connect.
func Open(ctx context.Context, cfg configs.MySQLConfig) (*Resource, error) {
	if ctx == nil {
		return nil, resources.NewCapabilityError(resources.CapabilityMySQL, "open", errors.New("nil context"))
	}
	if !cfg.Enabled {
		return &Resource{}, nil
	}

	dsn := buildDSN(cfg)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: newGORMLogger(os.Stdout, true)})
	if err != nil {
		return nil, resources.NewCapabilityError(resources.CapabilityMySQL, "open", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, resources.NewCapabilityError(resources.CapabilityMySQL, "db", err)
	}
	configurePool(sqlDB, cfg)

	return &Resource{enabled: true, db: db}, nil
}

// DB returns the underlying GORM DB, or nil when MySQL is disabled.
func (r *Resource) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

// Check returns the current MySQL capability status.
func (r *Resource) Check(ctx context.Context) resources.CapabilityStatus {
	if r == nil || !r.enabled {
		return resources.Disabled(resources.CapabilityMySQL)
	}
	sqlDB, err := r.sqlDB()
	if err != nil {
		return resources.Unavailable(resources.CapabilityMySQL, err)
	}
	if ctx == nil {
		return resources.Unavailable(resources.CapabilityMySQL, errors.New("nil context"))
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return resources.Unavailable(resources.CapabilityMySQL, err)
	}
	return resources.Available(resources.CapabilityMySQL, "ping ok")
}

// Close releases the MySQL connection pool.
func (r *Resource) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	sqlDB, err := r.sqlDB()
	if err != nil {
		return resources.NewCapabilityError(resources.CapabilityMySQL, "close", err)
	}
	if err := sqlDB.Close(); err != nil {
		return resources.NewCapabilityError(resources.CapabilityMySQL, "close", err)
	}
	return nil
}

func (r *Resource) sqlDB() (*sql.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("mysql db is nil")
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}
	if sqlDB == nil {
		return nil, fmt.Errorf("mysql sql db is nil")
	}
	return sqlDB, nil
}

func configurePool(db *sql.DB, cfg configs.MySQLConfig) {
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)
	}
}

func buildDSN(cfg configs.MySQLConfig) string {
	// Keep the static connection topology in config while allowing the password
	// to come from a secret-backed environment variable.
	driverConfig := drivermysql.NewConfig()
	driverConfig.User = cfg.Username
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	driverConfig.DBName = cfg.Database
	driverConfig.ParseTime = true
	driverConfig.Loc = time.Local
	driverConfig.Params = map[string]string{
		"charset": "utf8mb4",
	}
	return driverConfig.FormatDSN()
}

func newGORMLogger(writer io.Writer, colorful bool) logger.Interface {
	// Seed logic intentionally uses ErrRecordNotFound as an existence check; keep
	// real database warnings visible without printing those expected misses.
	return logger.New(stdlog.New(writer, "\r\n", stdlog.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  colorful,
	})
}
