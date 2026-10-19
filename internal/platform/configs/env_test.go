package configs

import (
	"slices"
	"testing"
)

func TestEnvOverridesDeriveFromStructTags(t *testing.T) {
	plan := envOverrides()

	bound := make(map[string]bool)
	for _, key := range plan.bindKeys {
		if bound[key] {
			t.Fatalf("bind key %q appears more than once", key)
		}
		bound[key] = true
		if envExcluded[key] {
			t.Fatalf("bind key %q is excluded but bound", key)
		}
	}

	for _, key := range []string{
		"app::name", "app::version", "system::port", "system::level",
		"log::caller", "admin::upload_dir", "http::secure_cookies",
		"http::cors::max_age_seconds", "mysql::enabled", "mysql::password",
		"mysql::max_open_conns", "redis::db", "mongodb::uri",
		"tracing::sample_ratio",
	} {
		if !bound[key] {
			t.Fatalf("bind keys missing %q, want every non-excluded leaf field derived from tags", key)
		}
	}
	for _, key := range []string{
		"mysql::host", "mysql::port", "mysql::database", "mysql::username",
	} {
		if bound[key] {
			t.Fatalf("bind keys contain %q, want file-only topology excluded", key)
		}
	}
}

func TestEnvOverridesCoverEveryStringSliceField(t *testing.T) {
	plan := envOverrides()

	names := make([]string, 0, len(plan.listTargets))
	for _, target := range plan.listTargets {
		names = append(names, target.name)
	}
	slices.Sort(names)

	want := []string{
		"ECHO_ADMIN_HTTP_CORS_ALLOW_HEADERS",
		"ECHO_ADMIN_HTTP_CORS_ALLOW_METHODS",
		"ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS",
		"ECHO_ADMIN_HTTP_CORS_EXPOSE_HEADERS",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("list targets = %v, want exactly the CORS list fields %v", names, want)
	}
}

func TestEnvVarNameMapsNestedKeys(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"app::name", "ECHO_ADMIN_APP_NAME"},
		{"http::cors::allow_origins", "ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := envVarName(tt.key); got != tt.want {
				t.Fatalf("envVarName(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// MySQL topology is file-only on purpose: even when the environment sets
// mysql::host, the file value wins.
func TestDecodeConfigWithEnvKeepsMySQLTopologyFileOnly(t *testing.T) {
	t.Setenv("ECHO_ADMIN_MYSQL_HOST", "env-host")
	t.Setenv("ECHO_ADMIN_MYSQL_DATABASE", "env-database")

	cfg, err := decodeConfigWithEnv([]byte(`
[mysql]
enabled = false
host = "file-host"
database = "file-database"
`), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if cfg.MySQL.Host != "file-host" {
		t.Fatalf("MySQL.Host = %q, want file-only file-host", cfg.MySQL.Host)
	}
	if cfg.MySQL.Database != "file-database" {
		t.Fatalf("MySQL.Database = %q, want file-only file-database", cfg.MySQL.Database)
	}
}

// mysql::enabled is not topology and not a secret, so it stays
// environment-overridable like every other capability switch.
func TestDecodeConfigWithEnvAppliesMySQLEnabledOverride(t *testing.T) {
	t.Setenv("ECHO_ADMIN_MYSQL_ENABLED", "false")

	cfg, err := decodeConfigWithEnv([]byte(`
[mysql]
enabled = true
host = "mysql"
port = 3306
database = "echo_admin"
username = "echo_admin"
`), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if cfg.MySQL.Enabled {
		t.Fatal("MySQL.Enabled = true, want env override false")
	}
}
