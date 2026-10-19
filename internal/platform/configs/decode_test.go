package configs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	envOnlyPort = ":9999"
)

func TestDecodeConfigWithEnvKeepsFileSourceOverrides(t *testing.T) {
	t.Setenv("ECHO_ADMIN_SYSTEM_PORT", envOnlyPort)
	t.Setenv("ECHO_ADMIN_APP_NAME", "payments-api")

	cfg, err := decodeConfigWithEnv([]byte(`
[app]
name = "echo-admin"

[system]
port = ":9322"
`), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if cfg.System.Port != envOnlyPort {
		t.Fatalf("System.Port = %q, want env override %s", cfg.System.Port, envOnlyPort)
	}
	if cfg.App.Name != "payments-api" {
		t.Fatalf("App.Name = %q, want payments-api", cfg.App.Name)
	}
}

func TestDecodeConfigWithEnvAppliesHTTPSecureCookiesOverride(t *testing.T) {
	t.Setenv("ECHO_ADMIN_HTTP_SECURE_COOKIES", "true")

	cfg, err := decodeConfigWithEnv([]byte(`
	[http]
	secure_cookies = false
	`), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if !cfg.HTTP.SecureCookies {
		t.Fatal("HTTP.SecureCookies = false, want env override true")
	}
}

func TestDecodeConfigWithEnvAppliesEnvOnlyValues(t *testing.T) {
	t.Setenv("ECHO_ADMIN_SYSTEM_PORT", envOnlyPort)
	t.Setenv("ECHO_ADMIN_LOG_FORMAT", "console")
	t.Setenv("ECHO_ADMIN_LOG_OUTPUT", "stderr")
	t.Setenv("ECHO_ADMIN_LOG_CALLER", "true")
	t.Setenv("ECHO_ADMIN_HTTP_ENABLE_GZIP", "false")
	t.Setenv("ECHO_ADMIN_HTTP_SECURE_COOKIES", "true")

	cfg, err := decodeConfigWithEnv([]byte(``), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if cfg.System.Port != envOnlyPort {
		t.Fatalf("System.Port = %q, want env-only %s", cfg.System.Port, envOnlyPort)
	}
	if cfg.Log.Format != LogFormatConsole {
		t.Fatalf("Log.Format = %q, want env-only console", cfg.Log.Format)
	}
	if cfg.Log.Output != LogOutputStderr {
		t.Fatalf("Log.Output = %q, want env-only stderr", cfg.Log.Output)
	}
	if !cfg.Log.Caller {
		t.Fatal("Log.Caller = false, want env-only true")
	}
	// The environment overrides the enabled-by-default layer.
	if cfg.HTTP.EnableGzip {
		t.Fatal("HTTP.EnableGzip = true, want env-only false")
	}
	// Keys absent from both file and environment fall back to the typed
	// defaultConfig layer: the middleware stays installed.
	if !cfg.HTTP.EnableRecovery {
		t.Fatal("HTTP.EnableRecovery = false, want default true")
	}
	if !cfg.HTTP.EnableRequestContext {
		t.Fatal("HTTP.EnableRequestContext = false, want default true")
	}
	if !cfg.HTTP.EnableRequestLog {
		t.Fatal("HTTP.EnableRequestLog = false, want default true")
	}
	if !cfg.HTTP.SecureCookies {
		t.Fatal("HTTP.SecureCookies = false, want env-only true")
	}
}

func TestDecodeConfigWithEnvAppliesCapabilityOverrides(t *testing.T) {
	t.Setenv("ECHO_ADMIN_MYSQL_PASSWORD", "env-db-password")
	t.Setenv("ECHO_ADMIN_REDIS_ENABLED", "true")
	t.Setenv("ECHO_ADMIN_REDIS_ADDRESS", "localhost:6379")
	t.Setenv("ECHO_ADMIN_MONGODB_ENABLED", "false")
	t.Setenv("ECHO_ADMIN_TRACING_ENABLED", "false")

	cfg, err := decodeConfigWithEnv([]byte(`
[mysql]
enabled = true
host = "mysql"
port = 3306
database = "app"
username = "app"
password = "file-db-password"

[redis]
enabled = false
address = ""

[mongodb]
enabled = false

[tracing]
enabled = false
`), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	if !cfg.MySQL.Enabled {
		t.Fatal("MySQL.Enabled = false, want file config true")
	}
	if cfg.MySQL.Host != "mysql" {
		t.Fatalf("MySQL.Host = %q, want file config mysql", cfg.MySQL.Host)
	}
	if cfg.MySQL.Password != "env-db-password" {
		t.Fatalf("MySQL.Password = %q, want env override", cfg.MySQL.Password)
	}
	if !cfg.Redis.Enabled {
		t.Fatal("Redis.Enabled = false, want env override true")
	}
	if cfg.Redis.Address != "localhost:6379" {
		t.Fatalf("Redis.Address = %q, want localhost:6379", cfg.Redis.Address)
	}
	if cfg.MongoDB.Enabled {
		t.Fatal("MongoDB.Enabled = true, want env override false")
	}
	if cfg.Tracing.Enabled {
		t.Fatal("Tracing.Enabled = true, want env override false")
	}
}

func TestDecodeConfigLoadsHTTPMiddlewareConfig(t *testing.T) {
	cfg, err := decodeConfigWithEnv([]byte(`
[http]
enable_recovery = false
enable_request_context = false
enable_request_log = false
enable_gzip = false
secure_cookies = true

[http.cors]
enabled = true
allow_origins = ["https://app.example.com"]
allow_methods = ["GET", "POST"]
allow_headers = ["Authorization", "Content-Type"]
allow_credentials = true
expose_headers = ["X-Request-ID"]
max_age_seconds = 600
`), "toml", false)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	// An explicit false in the file must win over the enabled-by-default
	// layer, otherwise opting out of a middleware would be impossible.
	if cfg.HTTP.EnableRecovery {
		t.Fatal("HTTP.EnableRecovery = true, want explicit file false")
	}
	if cfg.HTTP.EnableRequestContext {
		t.Fatal("HTTP.EnableRequestContext = true, want explicit file false")
	}
	if cfg.HTTP.EnableRequestLog {
		t.Fatal("HTTP.EnableRequestLog = true, want explicit file false")
	}
	if cfg.HTTP.EnableGzip {
		t.Fatal("HTTP.EnableGzip = true, want explicit file false")
	}
	if !cfg.HTTP.SecureCookies {
		t.Fatal("HTTP.SecureCookies = false, want true")
	}
	if !cfg.HTTP.CORS.Enabled {
		t.Fatal("HTTP.CORS.Enabled = false, want true")
	}
	if got := cfg.HTTP.CORS.AllowOrigins; len(got) != 1 || got[0] != "https://app.example.com" {
		t.Fatalf("HTTP.CORS.AllowOrigins = %#v, want app origin", got)
	}
	if cfg.HTTP.CORS.MaxAgeSeconds != 600 {
		t.Fatalf("HTTP.CORS.MaxAgeSeconds = %d, want 600", cfg.HTTP.CORS.MaxAgeSeconds)
	}
}

func TestDecodeConfigWithEnvAppliesListOverrides(t *testing.T) {
	t.Setenv("ECHO_ADMIN_HTTP_CORS_ENABLED", "true")
	t.Setenv("ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS", "https://app.example.com, https://admin.example.com")
	t.Setenv("ECHO_ADMIN_HTTP_CORS_ALLOW_METHODS", "GET,POST")
	t.Setenv("ECHO_ADMIN_HTTP_CORS_ALLOW_HEADERS", "Authorization,Content-Type")
	t.Setenv("ECHO_ADMIN_HTTP_CORS_EXPOSE_HEADERS", "X-Request-ID")

	cfg, err := decodeConfigWithEnv([]byte(``), "toml", true)
	if err != nil {
		t.Fatalf("decodeConfigWithEnv() error = %v", err)
	}

	assertStringSlice(t, cfg.HTTP.CORS.AllowOrigins, []string{"https://app.example.com", "https://admin.example.com"})
	assertStringSlice(t, cfg.HTTP.CORS.AllowMethods, []string{"GET", "POST"})
	assertStringSlice(t, cfg.HTTP.CORS.AllowHeaders, []string{"Authorization", "Content-Type"})
	assertStringSlice(t, cfg.HTTP.CORS.ExposeHeaders, []string{"X-Request-ID"})
}

func TestValidateRejectsEnabledCORSWithoutOrigins(t *testing.T) {
	err := validate(normalize(Config{
		HTTP: HTTPConfig{
			CORS: CORSConfig{
				Enabled: true,
			},
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want CORS config error")
	}
	if !strings.Contains(err.Error(), "cors") {
		t.Fatalf("validate() error = %q, want cors identified", err)
	}
}

func TestValidateRejectsWildcardCORSOriginWithCredentials(t *testing.T) {
	err := validate(normalize(Config{
		HTTP: HTTPConfig{
			CORS: CORSConfig{
				Enabled:          true,
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
			},
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want CORS wildcard credentials error")
	}
	if !strings.Contains(err.Error(), "wildcard") {
		t.Fatalf("validate() error = %q, want wildcard identified", err)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}

func TestLoadReadsFileAndAppliesEnvOverrides(t *testing.T) {
	t.Setenv("ECHO_ADMIN_SYSTEM_PORT", envOnlyPort)

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[system]
port = ":9322"
level = 1
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.System.Port != envOnlyPort {
		t.Fatalf("System.Port = %q, want env override %s", cfg.System.Port, envOnlyPort)
	}
	if cfg.System.Level != DebugLevel {
		t.Fatalf("System.Level = %d, want %d", cfg.System.Level, DebugLevel)
	}
	if cfg.Admin.UploadDir != defaultUploadDir {
		t.Fatalf("Admin.UploadDir = %q, want %q", cfg.Admin.UploadDir, defaultUploadDir)
	}
}

func TestLoadRejectsRetiredJWTConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[jwt]
enabled = true
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want retired jwt config rejected")
	}
}

func TestLoadRejectsUnsupportedConfigExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.conf")
	if err := os.WriteFile(path, []byte(`[system]`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want unsupported extension error")
	}
}

func TestLoadRejectsUnknownConfigFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[system]
port = ":9322"
unexpected = true
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want unknown field error")
	}
}

func TestValidateRejectsInvalidSystemLevel(t *testing.T) {
	err := validate(normalize(Config{
		System: SystemConfig{
			Level: 99,
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want invalid level error")
	}
}

func TestValidateRejectsInvalidLogConfig(t *testing.T) {
	err := validate(normalize(Config{
		Log: LogConfig{
			Format: "color",
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want invalid log format error")
	}

	err = validate(normalize(Config{
		Log: LogConfig{
			Output: "file",
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want invalid log output error")
	}
}

func TestValidateRejectsEnabledMongoDBWithoutURI(t *testing.T) {
	err := validate(normalize(Config{
		MongoDB: MongoDBConfig{
			Enabled: true,
		},
	}))
	if err == nil {
		t.Fatal("validate() error = nil, want enabled MongoDB without URI error")
	}
	if !strings.Contains(err.Error(), "mongodb") {
		t.Fatalf("validate() error = %q, want MongoDB capability identified", err)
	}
}

func TestValidateRejectsEnabledCapabilitiesWithoutRequiredConnectionSettings(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		capability string
	}{
		{
			name: "mysql",
			cfg: Config{
				MySQL: MySQLConfig{
					Enabled: true,
				},
			},
			capability: "mysql",
		},
		{
			name: "redis",
			cfg: Config{
				Redis: RedisConfig{
					Enabled: true,
				},
			},
			capability: "redis",
		},
		{
			name: "tracing",
			cfg: Config{
				Tracing: TracingConfig{
					Enabled: true,
				},
			},
			capability: "tracing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(normalize(tt.cfg))
			if err == nil {
				t.Fatal("validate() error = nil, want missing required setting error")
			}
			if !strings.Contains(err.Error(), tt.capability) {
				t.Fatalf("validate() error = %q, want %s capability identified", err, tt.capability)
			}
		})
	}
}

func TestNormalizeAcceptsTextAsConsoleLogFormatAlias(t *testing.T) {
	cfg := normalize(Config{
		Log: LogConfig{
			Format: "text",
		},
	})

	if cfg.Log.Format != LogFormatConsole {
		t.Fatalf("Log.Format = %q, want text alias normalized to console", cfg.Log.Format)
	}
}

func TestLoadTreatsExtensionlessFileInDottedDirectoryAsTOML(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config.d")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(`
[system]
port = ":9322"
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v, want extensionless TOML config to load", err)
	}
}
