package configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	cfg := normalize(Config{})

	assert.Equal(t, DefaultAppName, cfg.App.Name)
	assert.Equal(t, defaultAppVersion, cfg.App.Version)
	assert.Equal(t, DefaultPort, cfg.System.Port)
	assert.Equal(t, OnlineLevel, cfg.System.Level)

	assert.Equal(t, LogFormatJSON, cfg.Log.Format)
	assert.Equal(t, LogOutputStdout, cfg.Log.Output)
	assert.False(t, cfg.Log.Caller)

	assert.False(t, cfg.HTTP.SecureCookies)

	assert.Equal(t, defaultUploadDir, cfg.Admin.UploadDir)
	assert.False(t, cfg.MySQL.Enabled)
	assert.Equal(t, "", cfg.MySQL.Host)
	assert.Equal(t, defaultMySQLPort, cfg.MySQL.Port)
	assert.Equal(t, "", cfg.MySQL.Database)
	assert.Equal(t, "", cfg.MySQL.Username)
	assert.Equal(t, "", cfg.MySQL.Password)
	assert.Equal(t, defaultMySQLMaxOpenConns, cfg.MySQL.MaxOpenConns)
	assert.Equal(t, defaultMySQLMaxIdleConns, cfg.MySQL.MaxIdleConns)
	assert.Equal(t, defaultMySQLConnMaxLifetimeSeconds, cfg.MySQL.ConnMaxLifetimeSeconds)
	assert.Equal(t, defaultCapabilityTimeout, cfg.MySQL.PingTimeoutSeconds)

	assert.False(t, cfg.Redis.Enabled)
	assert.Equal(t, "", cfg.Redis.Address)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, defaultCapabilityTimeout, cfg.Redis.PingTimeoutSeconds)

	assert.False(t, cfg.MongoDB.Enabled)
	assert.Equal(t, "", cfg.MongoDB.URI)
	assert.Equal(t, defaultCapabilityTimeout, cfg.MongoDB.ConnectTimeoutSeconds)
	assert.Equal(t, defaultCapabilityTimeout, cfg.MongoDB.PingTimeoutSeconds)

	assert.False(t, cfg.Tracing.Enabled)
	assert.Equal(t, "", cfg.Tracing.Endpoint)
	assert.Equal(t, defaultTracingProtocol, cfg.Tracing.Protocol)
	assert.Equal(t, float64(0), cfg.Tracing.SampleRatio)
	assert.Equal(t, defaultTracingShutdownTimeoutSeconds, cfg.Tracing.ShutdownTimeoutSeconds)
}

// Debug mode defaults to console-format logs so local runs stay readable.
func TestConfigDefaultsDebugModeUsesConsoleLogFormat(t *testing.T) {
	cfg := normalize(Config{
		System: SystemConfig{Level: DebugLevel},
	})

	assert.Equal(t, LogFormatConsole, cfg.Log.Format)
}

func TestAppConfig(t *testing.T) {
	cfg := AppConfig{
		Name:    "billing-api",
		Version: "2026.06.17",
	}

	assert.Equal(t, "billing-api", cfg.Name)
	assert.Equal(t, "2026.06.17", cfg.Version)
}

func TestSystemConfig(t *testing.T) {
	cfg := SystemConfig{
		Port: "8080",
	}

	assert.Equal(t, "8080", cfg.Port)
}

func TestLogConfig(t *testing.T) {
	cfg := LogConfig{
		Format: LogFormatConsole,
		Output: LogOutputStderr,
		Caller: true,
	}

	assert.Equal(t, LogFormatConsole, cfg.Format)
	assert.Equal(t, LogOutputStderr, cfg.Output)
	assert.True(t, cfg.Caller)
}

func TestHTTPConfig(t *testing.T) {
	cfg := HTTPConfig{
		SecureCookies: true,
	}

	assert.True(t, cfg.SecureCookies)
}

func TestCapabilityConfig(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     defaultMySQLPort,
			Database: "app",
			Username: "user",
			Password: "pass",
		},
		Redis: RedisConfig{
			Enabled: true,
			Address: "localhost:6379",
		},
		MongoDB: MongoDBConfig{
			Enabled: false,
		},
		Tracing: TracingConfig{
			Enabled: false,
		},
	}

	assert.True(t, cfg.MySQL.Enabled)
	assert.True(t, cfg.Redis.Enabled)
	assert.False(t, cfg.MongoDB.Enabled)
	assert.False(t, cfg.Tracing.Enabled)
}

func TestValidateRejectsBlankAppIdentity(t *testing.T) {
	err := validate(Config{
		App: AppConfig{
			Name:    " ",
			Version: "dev",
		},
	})

	assert.Error(t, err)

	err = validate(Config{
		App: AppConfig{
			Name:    "echo-admin",
			Version: " ",
		},
	})

	assert.Error(t, err)
}
