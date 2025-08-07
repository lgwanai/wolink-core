package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
	Models   ModelsConfig   `mapstructure:"models"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Type     string `mapstructure:"type"`     // mysql 或 postgres
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`  // 仅用于 PostgreSQL
	Charset  string `mapstructure:"charset"`  // 仅用于 MySQL
	ParseTime bool  `mapstructure:"parsetime"` // 仅用于 MySQL
	Loc      string `mapstructure:"loc"`      // 仅用于 MySQL
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
		`\d{11}`,           // 手机号
		`\d{18}|\d{15}`,    // 身份证号
		`\d{16}`,           // 银行卡号
	})
	viper.SetDefault("security.replacement_patterns", []string{
		"[PHONE]",
		"[ID_CARD]",
		"[BANK_CARD]",
	})
	
	viper.SetDefault("models.config_path", "./configs/models")
}