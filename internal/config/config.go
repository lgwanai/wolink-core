package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server           ServerConfig           `mapstructure:"server"`
	Database         DatabaseConfig         `mapstructure:"database"`
	Redis            RedisConfig            `mapstructure:"redis"`
	Log              LogConfig              `mapstructure:"log"`
	Security         SecurityConfig         `mapstructure:"security"`
	Models           ModelsConfig           `mapstructure:"models"`
	Infrastructure   InfrastructureConfig   `mapstructure:"infrastructure"`
	CommunicationLog CommunicationLogConfig `mapstructure:"communication_log"`
	Node             NodeConfig             `mapstructure:"node"`
	Admin            AdminConfig            `mapstructure:"admin"`
}

// AdminConfig holds admin API authentication settings
type AdminConfig struct {
	Token string `mapstructure:"token"` // Admin API Token (at least 32 chars)
}

// InfrastructureConfig holds infrastructure-level settings for graceful shutdown,
// HTTP server timeouts, and connection pool configurations.
type InfrastructureConfig struct {
	ShutdownTimeout   time.Duration   `mapstructure:"shutdown_timeout"`
	ReadTimeout       time.Duration   `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration   `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration   `mapstructure:"idle_timeout"`
	ReadHeaderTimeout time.Duration   `mapstructure:"read_header_timeout"`
	Database          DBPoolConfig    `mapstructure:"database_pool"`
	Redis             RedisPoolConfig `mapstructure:"redis_pool"`
}

// DBPoolConfig holds database connection pool settings.
type DBPoolConfig struct {
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// RedisPoolConfig holds Redis connection pool settings.
type RedisPoolConfig struct {
	PoolSize        int           `mapstructure:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	PoolTimeout     time.Duration `mapstructure:"pool_timeout"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Type      string `mapstructure:"type"` // mysql 或 postgres
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	User      string `mapstructure:"user"`
	Password  string `mapstructure:"password"`
	DBName    string `mapstructure:"dbname"`
	SSLMode   string `mapstructure:"sslmode"`   // 仅用于 PostgreSQL
	Charset   string `mapstructure:"charset"`   // 仅用于 MySQL
	ParseTime bool   `mapstructure:"parsetime"` // 仅用于 MySQL
	Loc       string `mapstructure:"loc"`       // 仅用于 MySQL
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type SecurityConfig struct {
	JWTSecret           string   `mapstructure:"jwt_secret"`
	SensitivePatterns   []string `mapstructure:"sensitive_patterns"`
	ReplacementPatterns []string `mapstructure:"replacement_patterns"`
}

type ModelsConfig struct {
	ConfigPath string `mapstructure:"config_path"`
}

// CommunicationLogConfig holds settings for LLM communication persistence
type CommunicationLogConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	Mode             string `mapstructure:"mode"`               // local, shared, disabled
	StoragePath      string `mapstructure:"storage_path"`
	EnableRemoteSync bool   `mapstructure:"enable_remote_sync"` // 是否同步到Redis队列
	RedisQueueKey    string `mapstructure:"redis_queue_key"`
	MaxFileSizeMB    int64  `mapstructure:"max_file_size_mb"`
	FlushIntervalMs  int    `mapstructure:"flush_interval_ms"`
}

// NodeConfig holds node-specific configuration for distributed deployment
type NodeConfig struct {
	ID     string `mapstructure:"id"`     // 节点唯一标识
	Region string `mapstructure:"region"` // 区域标识（可选）
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置环境变量前缀
	viper.SetEnvPrefix("AI_GATEWAY")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用默认值
			fmt.Println("Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

func setDefaults() {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")

	viper.SetDefault("database.type", "mysql")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.dbname", "ai_gateway")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.charset", "utf8mb4")
	viper.SetDefault("database.parsetime", true)
	viper.SetDefault("database.loc", "Local")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("log.level", "info")

	viper.SetDefault("security.jwt_secret", "your-secret-key")
	viper.SetDefault("security.sensitive_patterns", []string{
		`\d{11}`,        // 手机号
		`\d{18}|\d{15}`, // 身份证号
		`\d{16}`,        // 银行卡号
	})
	viper.SetDefault("security.replacement_patterns", []string{
		"[PHONE]",
		"[ID_CARD]",
		"[BANK_CARD]",
	})

	viper.SetDefault("models.config_path", "./configs/models")

	// Node defaults
	viper.SetDefault("node.id", "node-default")
	viper.SetDefault("node.region", "default")

	// Communication log defaults
	viper.SetDefault("communication_log.enabled", false)
	viper.SetDefault("communication_log.storage_path", "./logs/communications")
	viper.SetDefault("communication_log.max_file_size_mb", 100)
	viper.SetDefault("communication_log.mode", "local")
	viper.SetDefault("communication_log.enable_remote_sync", false)
	viper.SetDefault("communication_log.redis_queue_key", "wolink:comm_logs")
	viper.SetDefault("communication_log.flush_interval_ms", 1000)

	// Infrastructure defaults
	viper.SetDefault("infrastructure.shutdown_timeout", "30s")
	viper.SetDefault("infrastructure.read_timeout", "15s")
	viper.SetDefault("infrastructure.write_timeout", "30s")
	viper.SetDefault("infrastructure.idle_timeout", "120s")
	viper.SetDefault("infrastructure.read_header_timeout", "5s")

	// Database pool defaults
	viper.SetDefault("infrastructure.database_pool.max_open_conns", 25)
	viper.SetDefault("infrastructure.database_pool.max_idle_conns", 10)
	viper.SetDefault("infrastructure.database_pool.conn_max_lifetime", "5m")

	// Redis pool defaults
	viper.SetDefault("infrastructure.redis_pool.pool_size", 20)
	viper.SetDefault("infrastructure.redis_pool.min_idle_conns", 5)
	viper.SetDefault("infrastructure.redis_pool.conn_max_lifetime", "5m")

	// Admin defaults
	viper.SetDefault("admin.token", "")
}
