package configs

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

const (
	configFormatJSON = "json"
	configFormatTOML = "toml"
	configFormatYAML = "yaml"
	configFormatYML  = "yml"
)

// envExcluded lists config keys that must stay file-only. MySQL topology
// (host, port, database, username) belongs in the versioned config file;
// only the password may come from the environment, so a deployment keeps
// topology in the file while credentials never enter it.
var envExcluded = map[string]bool{
	"mysql::host":     true,
	"mysql::port":     true,
	"mysql::database": true,
	"mysql::username": true,
}

// envListTarget points one []string config field at its environment variable.
type envListTarget struct {
	name      string
	fieldPath []int
}

// envOverridePlan describes how environment overrides reach Config fields.
// bindKeys go through viper BindEnv so Unmarshal sees them; []string fields
// are overridden directly from the environment because viper's comma
// splitting neither trims whitespace nor drops empty entries.
type envOverridePlan struct {
	bindKeys    []string
	listTargets []envListTarget
}

// envOverrides derives the plan from Config struct tags once per process.
// The mapstructure tags are the single source of truth: adding a config
// field makes it environment-overridable without any extra registration.
var envOverrides = sync.OnceValue(deriveEnvOverrides)

func deriveEnvOverrides() envOverridePlan {
	var plan envOverridePlan
	walkEnvFields(reflect.TypeOf(Config{}), nil, nil, &plan)
	return plan
}

func walkEnvFields(t reflect.Type, keyPrefix []string, fieldPath []int, plan *envOverridePlan) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}
		keys := append(append([]string{}, keyPrefix...), tag)
		path := append(append([]int{}, fieldPath...), i)
		key := strings.Join(keys, "::")
		switch {
		case field.Type.Kind() == reflect.Struct:
			walkEnvFields(field.Type, keys, path, plan)
		case field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.String:
			if !envExcluded[key] {
				plan.listTargets = append(plan.listTargets, envListTarget{name: envVarName(key), fieldPath: path})
			}
		case !envExcluded[key]:
			plan.bindKeys = append(plan.bindKeys, key)
		}
	}
}

// envVarName maps a config key such as "http::cors::allow_origins" to its
// environment variable ECHO_ADMIN_HTTP_CORS_ALLOW_ORIGINS.
func envVarName(key string) string {
	name := strings.NewReplacer("::", "_", ".", "_").Replace(key)
	return envPrefix + "_" + strings.ToUpper(name)
}

func decodeConfigWithEnv(data []byte, format string, useEnv bool) (Config, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.SetConfigType(configType(format))
	applyConfigDefaults(v)
	plan := envOverrides()
	if useEnv {
		// No viper AutomaticEnv here: it would let the environment override
		// any key present in the config file, including the file-only MySQL
		// topology keys. Only struct-tag-derived keys (minus envExcluded)
		// are bound and therefore overridable.
		v.SetEnvPrefix(envPrefix)
		v.SetEnvKeyReplacer(strings.NewReplacer("::", "_", ".", "_"))
		for _, key := range plan.bindKeys {
			if err := v.BindEnv(key); err != nil {
				return Config{}, err
			}
		}
	}
	if err := v.ReadConfig(bytes.NewBuffer(data)); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return Config{}, err
	}
	if useEnv {
		applyEnvListOverrides(&cfg, plan.listTargets)
	}
	cfg = normalize(cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyConfigDefaults installs defaultConfig's non-zero leaves as the viper
// default layer so positive booleans can default to enabled while an explicit
// false in the config file still wins and the environment overrides both.
// Keys derive from the mapstructure tags — the same single source the
// environment override plan uses, so no key is registered by hand.
func applyConfigDefaults(v *viper.Viper) {
	walkDefaultValues(reflect.ValueOf(defaultConfig), nil, v)
}

func walkDefaultValues(value reflect.Value, keyPrefix []string, v *viper.Viper) {
	t := value.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}
		keys := append(append([]string{}, keyPrefix...), tag)
		fieldValue := value.Field(i)
		if field.Type.Kind() == reflect.Struct {
			walkDefaultValues(fieldValue, keys, v)
			continue
		}
		if !fieldValue.IsZero() {
			v.SetDefault(strings.Join(keys, "::"), fieldValue.Interface())
		}
	}
}

// normalize applies application config defaults.
func normalize(cfg Config) Config {
	cfg = normalizeAppDefaults(cfg)
	cfg = normalizeLogDefaults(cfg)
	cfg = normalizeAdminDefaults(cfg)
	cfg = normalizeResourceDefaults(cfg)
	return normalizeTracingDefaults(cfg)
}

func normalizeAppDefaults(cfg Config) Config {
	if cfg.App.Name == "" {
		cfg.App.Name = DefaultAppName
	}
	if cfg.App.Version == "" {
		cfg.App.Version = defaultAppVersion
	}
	if cfg.System.Port == "" {
		cfg.System.Port = DefaultPort
	}
	if cfg.System.Level == 0 {
		cfg.System.Level = OnlineLevel
	}
	return cfg
}

func normalizeLogDefaults(cfg Config) Config {
	cfg.Log.Format = strings.ToLower(strings.TrimSpace(cfg.Log.Format))
	if cfg.Log.Format == "" {
		cfg.Log.Format = LogFormatJSON
		if cfg.System.Level == DebugLevel {
			cfg.Log.Format = LogFormatConsole
		}
	}
	if cfg.Log.Format == "text" {
		cfg.Log.Format = LogFormatConsole
	}
	cfg.Log.Output = strings.ToLower(strings.TrimSpace(cfg.Log.Output))
	if cfg.Log.Output == "" {
		cfg.Log.Output = LogOutputStdout
	}
	return cfg
}

func normalizeAdminDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.Admin.UploadDir) == "" {
		cfg.Admin.UploadDir = defaultUploadDir
	}
	return cfg
}

func normalizeResourceDefaults(cfg Config) Config {
	cfg.MySQL.Host = strings.TrimSpace(cfg.MySQL.Host)
	cfg.MySQL.Database = strings.TrimSpace(cfg.MySQL.Database)
	cfg.MySQL.Username = strings.TrimSpace(cfg.MySQL.Username)
	cfg.MySQL.Password = strings.TrimSpace(cfg.MySQL.Password)
	if cfg.MySQL.Port == 0 {
		cfg.MySQL.Port = defaultMySQLPort
	}
	if cfg.MySQL.MaxOpenConns == 0 {
		cfg.MySQL.MaxOpenConns = defaultMySQLMaxOpenConns
	}
	if cfg.MySQL.MaxIdleConns == 0 {
		cfg.MySQL.MaxIdleConns = defaultMySQLMaxIdleConns
	}
	if cfg.MySQL.ConnMaxLifetimeSeconds == 0 {
		cfg.MySQL.ConnMaxLifetimeSeconds = defaultMySQLConnMaxLifetimeSeconds
	}
	if cfg.MySQL.PingTimeoutSeconds == 0 {
		cfg.MySQL.PingTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.Redis.DialTimeoutSeconds == 0 {
		cfg.Redis.DialTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.Redis.ReadTimeoutSeconds == 0 {
		cfg.Redis.ReadTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.Redis.WriteTimeoutSeconds == 0 {
		cfg.Redis.WriteTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.Redis.PingTimeoutSeconds == 0 {
		cfg.Redis.PingTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.MongoDB.ConnectTimeoutSeconds == 0 {
		cfg.MongoDB.ConnectTimeoutSeconds = defaultCapabilityTimeout
	}
	if cfg.MongoDB.PingTimeoutSeconds == 0 {
		cfg.MongoDB.PingTimeoutSeconds = defaultCapabilityTimeout
	}
	return cfg
}

func normalizeTracingDefaults(cfg Config) Config {
	cfg.Tracing.Protocol = strings.ToLower(strings.TrimSpace(cfg.Tracing.Protocol))
	if cfg.Tracing.Protocol == "" {
		cfg.Tracing.Protocol = defaultTracingProtocol
	}
	if cfg.Tracing.ShutdownTimeoutSeconds == 0 {
		cfg.Tracing.ShutdownTimeoutSeconds = defaultTracingShutdownTimeoutSeconds
	}
	return cfg
}

// validate checks cross-field configuration rules on an already-normalized
// config. Callers must run normalize first; validating a raw partial Config
// reports unrelated missing defaults instead of the intended rule.
func validate(cfg Config) error {
	if err := validateAppConfig(cfg.App); err != nil {
		return err
	}
	if err := validateSystemConfig(cfg.System); err != nil {
		return err
	}
	if err := validateLogConfig(cfg.Log); err != nil {
		return err
	}
	if err := validateAdminConfig(cfg.Admin); err != nil {
		return err
	}
	return validateConfiguredResources(cfg)
}

func applyEnvListOverrides(cfg *Config, targets []envListTarget) {
	if cfg == nil {
		return
	}
	for _, target := range targets {
		raw, ok := os.LookupEnv(target.name)
		if !ok {
			continue
		}
		reflect.ValueOf(cfg).Elem().FieldByIndex(target.fieldPath).Set(reflect.ValueOf(splitEnvList(raw)))
	}
}

func splitEnvList(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func validateAppConfig(cfg AppConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("app name is required")
	}
	if strings.TrimSpace(cfg.Version) == "" {
		return errors.New("app version is required")
	}
	return nil
}

func validateSystemConfig(cfg SystemConfig) error {
	switch cfg.Level {
	case DebugLevel, OnlineLevel:
		return nil
	default:
		return errors.New("system level must be 1 (debug) or 2 (online)")
	}
}

func validateLogConfig(cfg LogConfig) error {
	switch cfg.Format {
	case LogFormatConsole, LogFormatJSON:
	default:
		return errors.New("log format must be console or json")
	}
	switch cfg.Output {
	case LogOutputStdout, LogOutputStderr:
		return nil
	default:
		return errors.New("log output must be stdout or stderr")
	}
}

func validateAdminConfig(cfg AdminConfig) error {
	if strings.TrimSpace(cfg.UploadDir) == "" {
		return errors.New("admin upload_dir is required")
	}
	return nil
}

func validateConfiguredResources(cfg Config) error {
	if err := validateHTTPConfig(cfg.HTTP); err != nil {
		return err
	}
	if err := validateMySQLConfig(cfg.MySQL); err != nil {
		return err
	}
	if err := validateRedisConfig(cfg.Redis); err != nil {
		return err
	}
	if err := validateMongoDBConfig(cfg.MongoDB); err != nil {
		return err
	}
	return validateTracingConfig(cfg.Tracing)
}

func validateHTTPConfig(cfg HTTPConfig) error {
	if cfg.CORS.MaxAgeSeconds < 0 {
		return errors.New("http cors max_age_seconds must not be negative")
	}
	if cfg.CORS.Enabled && len(cfg.CORS.AllowOrigins) == 0 {
		return errors.New("http cors allow_origins are required when cors is enabled")
	}
	if cfg.CORS.Enabled && cfg.CORS.AllowCredentials && hasWildcardOrigin(cfg.CORS.AllowOrigins) {
		return errors.New("http cors allow_origins must not include wildcard when allow_credentials is enabled")
	}
	return nil
}

func hasWildcardOrigin(origins []string) bool {
	for _, origin := range origins {
		if strings.TrimSpace(origin) == "*" {
			return true
		}
	}
	return false
}

func validateMySQLConfig(cfg MySQLConfig) error {
	if cfg.Port < 0 || cfg.Port > 65535 {
		return errors.New("mysql port must be between 1 and 65535")
	}
	if cfg.MaxOpenConns < 0 {
		return errors.New("mysql max_open_conns must not be negative")
	}
	if cfg.MaxIdleConns < 0 {
		return errors.New("mysql max_idle_conns must not be negative")
	}
	if cfg.ConnMaxLifetimeSeconds < 0 {
		return errors.New("mysql conn_max_lifetime_seconds must not be negative")
	}
	if cfg.PingTimeoutSeconds < 0 {
		return errors.New("mysql ping_timeout_seconds must not be negative")
	}
	if cfg.Enabled && cfg.Port == 0 {
		return errors.New("mysql port is required when mysql is enabled")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Host) == "" {
		return errors.New("mysql host is required when mysql is enabled")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Database) == "" {
		return errors.New("mysql database is required when mysql is enabled")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Username) == "" {
		return errors.New("mysql username is required when mysql is enabled")
	}
	if cfg.Enabled && cfg.MaxIdleConns > cfg.MaxOpenConns {
		return errors.New("mysql max_idle_conns must not exceed max_open_conns")
	}
	return nil
}

func validateRedisConfig(cfg RedisConfig) error {
	if cfg.DB < 0 {
		return errors.New("redis db must not be negative")
	}
	if cfg.DialTimeoutSeconds < 0 {
		return errors.New("redis dial_timeout_seconds must not be negative")
	}
	if cfg.ReadTimeoutSeconds < 0 {
		return errors.New("redis read_timeout_seconds must not be negative")
	}
	if cfg.WriteTimeoutSeconds < 0 {
		return errors.New("redis write_timeout_seconds must not be negative")
	}
	if cfg.PingTimeoutSeconds < 0 {
		return errors.New("redis ping_timeout_seconds must not be negative")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Address) == "" {
		return errors.New("redis address is required when redis is enabled")
	}
	return nil
}

func validateMongoDBConfig(cfg MongoDBConfig) error {
	if cfg.ConnectTimeoutSeconds < 0 {
		return errors.New("mongodb connect_timeout_seconds must not be negative")
	}
	if cfg.PingTimeoutSeconds < 0 {
		return errors.New("mongodb ping_timeout_seconds must not be negative")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.URI) == "" {
		return errors.New("mongodb uri is required when mongodb is enabled")
	}
	return nil
}

func validateTracingConfig(cfg TracingConfig) error {
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return errors.New("tracing sample_ratio must be between 0 and 1")
	}
	if cfg.ShutdownTimeoutSeconds < 0 {
		return errors.New("tracing shutdown_timeout_seconds must not be negative")
	}
	switch cfg.Protocol {
	case "grpc", "http":
	default:
		return errors.New("tracing protocol must be grpc or http")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Endpoint) == "" {
		return errors.New("tracing endpoint is required when tracing is enabled")
	}
	return nil
}

func configType(format string) string {
	switch strings.ToLower(format) {
	case configFormatJSON:
		return configFormatJSON
	case configFormatYAML, configFormatYML:
		return configFormatYAML
	default:
		return configFormatTOML
	}
}
