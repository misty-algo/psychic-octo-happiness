// Package configs loads and validates static application configuration.
package configs

// Level identifies the runtime logging and debug mode.
type Level int8

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel Level = iota + 1
	// OnlineLevel is the default production priority.
	OnlineLevel
)

const (
	// DefaultAppName is returned by /api/info when config omits app.name.
	DefaultAppName = "echo-admin"

	// DefaultPort is the HTTP port used when config omits system.port.
	DefaultPort = ":9322"

	// LogFormatConsole writes human-readable zerolog console output.
	LogFormatConsole = "console"

	// LogFormatJSON writes structured zerolog JSON output.
	LogFormatJSON = "json"

	// LogOutputStdout writes logs to standard output.
	LogOutputStdout = "stdout"

	// LogOutputStderr writes logs to standard error.
	LogOutputStderr = "stderr"
)

// envPrefix is the environment variable prefix used for config overrides.
const envPrefix = "ECHO_ADMIN"

const (
	defaultAppVersion        = "dev"
	defaultUploadDir         = "uploads"
	defaultCapabilityTimeout = 3

	defaultMySQLMaxOpenConns           = 25
	defaultMySQLMaxIdleConns           = 5
	defaultMySQLConnMaxLifetimeSeconds = 300
	defaultMySQLPort                   = 3306

	defaultTracingProtocol               = "grpc"
	defaultTracingShutdownTimeoutSeconds = 5
)

// Config is the complete application configuration loaded at startup.
type Config struct {
	App     AppConfig     `mapstructure:"app"`
	System  SystemConfig  `mapstructure:"system"`
	Log     LogConfig     `mapstructure:"log"`
	HTTP    HTTPConfig    `mapstructure:"http"`
	Admin   AdminConfig   `mapstructure:"admin"`
	MySQL   MySQLConfig   `mapstructure:"mysql"`
	Redis   RedisConfig   `mapstructure:"redis"`
	MongoDB MongoDBConfig `mapstructure:"mongodb"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

// AdminConfig controls back-office runtime behavior.
type AdminConfig struct {
	UploadDir string `mapstructure:"upload_dir"`
}

// AppConfig controls process identity exposed by system routes.
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

// SystemConfig controls process-level runtime settings.
type SystemConfig struct {
	Port  string `mapstructure:"port"`
	Level Level  `mapstructure:"level"`
}

// LogConfig controls process-level structured logging.
type LogConfig struct {
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	Caller bool   `mapstructure:"caller"`
}

// HTTPConfig controls server-owned HTTP middleware. The Enable* booleans
// default to true via defaultConfig: omitting a key keeps the middleware
// installed, an explicit false opts out, and the environment can override
// both.
type HTTPConfig struct {
	EnableRecovery       bool       `mapstructure:"enable_recovery"`
	EnableRequestContext bool       `mapstructure:"enable_request_context"`
	EnableRequestLog     bool       `mapstructure:"enable_request_log"`
	EnableGzip           bool       `mapstructure:"enable_gzip"`
	SecureCookies        bool       `mapstructure:"secure_cookies"`
	CORS                 CORSConfig `mapstructure:"cors"`
}

// defaultConfig declares typed defaults installed as the viper default
// layer (priority: environment > config file > defaults). Only non-zero
// leaves apply; keys derive from the same mapstructure tags the environment
// override plan uses.
var defaultConfig = Config{
	HTTP: HTTPConfig{
		EnableRecovery:       true,
		EnableRequestContext: true,
		EnableRequestLog:     true,
		EnableGzip:           true,
	},
}

// CORSConfig controls optional CORS middleware.
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	MaxAgeSeconds    int      `mapstructure:"max_age_seconds"`
}

// MySQLConfig controls the optional process-level MySQL resource.
type MySQLConfig struct {
	Enabled                bool   `mapstructure:"enabled"`
	Host                   string `mapstructure:"host"`
	Port                   int    `mapstructure:"port"`
	Database               string `mapstructure:"database"`
	Username               string `mapstructure:"username"`
	Password               string `mapstructure:"password"`
	MaxOpenConns           int    `mapstructure:"max_open_conns"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `mapstructure:"conn_max_lifetime_seconds"`
	PingTimeoutSeconds     int    `mapstructure:"ping_timeout_seconds"`
}

// RedisConfig controls the optional process-level Redis resource.
type RedisConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	Address             string `mapstructure:"address"`
	Username            string `mapstructure:"username"`
	Password            string `mapstructure:"password"`
	DB                  int    `mapstructure:"db"`
	DialTimeoutSeconds  int    `mapstructure:"dial_timeout_seconds"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
	PingTimeoutSeconds  int    `mapstructure:"ping_timeout_seconds"`
}

// MongoDBConfig controls the optional process-level MongoDB resource.
type MongoDBConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	URI                   string `mapstructure:"uri"`
	Database              string `mapstructure:"database"`
	ConnectTimeoutSeconds int    `mapstructure:"connect_timeout_seconds"`
	PingTimeoutSeconds    int    `mapstructure:"ping_timeout_seconds"`
}

// TracingConfig controls optional OpenTelemetry export to Jaeger OTLP.
type TracingConfig struct {
	Enabled                bool    `mapstructure:"enabled"`
	Endpoint               string  `mapstructure:"endpoint"`
	Protocol               string  `mapstructure:"protocol"`
	Insecure               bool    `mapstructure:"insecure"`
	ServiceName            string  `mapstructure:"service_name"`
	SampleRatio            float64 `mapstructure:"sample_ratio"`
	ShutdownTimeoutSeconds int     `mapstructure:"shutdown_timeout_seconds"`
}
