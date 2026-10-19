package mysql

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/platform/configs"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/resources"
)

func TestGORMLoggerIgnoresRecordNotFound(t *testing.T) {
	var output bytes.Buffer
	gormLogger := newGORMLogger(&output, false)

	gormLogger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT * FROM access_menus WHERE path = '/dashboard'", 0
	}, gorm.ErrRecordNotFound)

	if got := output.String(); got != "" {
		t.Fatalf("logger output = %q, want empty output for record-not-found seed check", got)
	}
}

func TestGORMLoggerReportsDatabaseErrors(t *testing.T) {
	var output bytes.Buffer
	gormLogger := newGORMLogger(&output, false)
	dbErr := errors.New("connection refused")

	gormLogger.Trace(context.Background(), time.Now(), func() (string, int64) {
		return "SELECT * FROM access_menus", 0
	}, dbErr)

	got := output.String()
	if !strings.Contains(got, dbErr.Error()) {
		t.Fatalf("logger output = %q, want database error %q", got, dbErr)
	}
}

func TestOpenDisabledMySQLDoesNotConnect(t *testing.T) {
	resource, err := Open(context.Background(), configs.MySQLConfig{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if resource == nil {
		t.Fatal("Open() resource = nil, want disabled resource")
	}
	if resource.DB() != nil {
		t.Fatal("DB() != nil, want disabled resource without DB")
	}

	status := resource.Check(context.Background())
	if status.State != resources.StateDisabled {
		t.Fatalf("State = %q, want %q", status.State, resources.StateDisabled)
	}
}

func TestBuildDSNUsesStructuredMySQLConfig(t *testing.T) {
	dsn := buildDSN(configs.MySQLConfig{
		Host:     "mysql",
		Port:     3306,
		Database: "echo_admin",
		Username: "echo_admin",
		Password: "secret-password",
	})

	parsed, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN() error = %v", err)
	}
	if parsed.User != "echo_admin" {
		t.Fatalf("User = %q, want echo_admin", parsed.User)
	}
	if parsed.Passwd != "secret-password" {
		t.Fatalf("Passwd = %q, want secret-password", parsed.Passwd)
	}
	if parsed.Addr != "mysql:3306" {
		t.Fatalf("Addr = %q, want mysql:3306", parsed.Addr)
	}
	if parsed.DBName != "echo_admin" {
		t.Fatalf("DBName = %q, want echo_admin", parsed.DBName)
	}
	if !parsed.ParseTime {
		t.Fatal("ParseTime = false, want true")
	}
	if parsed.Params["charset"] != "utf8mb4" {
		t.Fatalf("charset = %q, want utf8mb4", parsed.Params["charset"])
	}
}

func TestCheckEnabledMySQLWithoutDBReportsUnavailable(t *testing.T) {
	resource := &Resource{enabled: true}

	status := resource.Check(context.Background())
	if status.Name != resources.CapabilityMySQL {
		t.Fatalf("Name = %q, want %q", status.Name, resources.CapabilityMySQL)
	}
	if !status.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if status.Available {
		t.Fatal("Available = true, want false")
	}
	if status.State != resources.StateUnavailable {
		t.Fatalf("State = %q, want %q", status.State, resources.StateUnavailable)
	}
}

func TestCloseDisabledMySQLIsNoop(t *testing.T) {
	resource := &Resource{}

	if err := resource.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
