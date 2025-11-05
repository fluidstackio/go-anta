package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Log      LogConfig      `yaml:"log" json:"log"`
	Device   DeviceConfig   `yaml:"device" json:"device"`
	Test     TestConfig     `yaml:"test" json:"test"`
	Reporter ReporterConfig `yaml:"reporter" json:"reporter"`
}

type LogConfig struct {
	Level string `yaml:"level" json:"level"`
	File  string `yaml:"file" json:"file"`
}

type DeviceConfig struct {
	Timeout        time.Duration `yaml:"timeout" json:"timeout"`
	MaxConnections int           `yaml:"max_connections" json:"max_connections"`
	RetryAttempts  int           `yaml:"retry_attempts" json:"retry_attempts"`
	RetryDelay     time.Duration `yaml:"retry_delay" json:"retry_delay"`
}

type TestConfig struct {
	MaxConcurrency int           `yaml:"max_concurrency" json:"max_concurrency"`
	CacheTTL       time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	CacheSize      int           `yaml:"cache_size" json:"cache_size"`
}

type ReporterConfig struct {
	DefaultFormat string `yaml:"default_format" json:"default_format"`
	OutputFile    string `yaml:"output_file" json:"output_file"`
}

var defaultConfig = Config{
	Log: LogConfig{
		Level: "info",
		File:  "",
	},
	Device: DeviceConfig{
		Timeout:        30 * time.Second,
		MaxConnections: 100,
		RetryAttempts:  3,
		RetryDelay:     5 * time.Second,
	},
	Test: TestConfig{
		MaxConcurrency: 10,
		CacheTTL:       60 * time.Second,
		CacheSize:      128,
	},
	Reporter: ReporterConfig{
		DefaultFormat: "table",
		OutputFile:    "",
	},
}

func LoadConfig() (*Config, error) {
	cfg := defaultConfig

	viper.SetDefault("log.level", cfg.Log.Level)
	viper.SetDefault("log.file", cfg.Log.File)
	viper.SetDefault("device.timeout", cfg.Device.Timeout)
	viper.SetDefault("device.max_connections", cfg.Device.MaxConnections)
	viper.SetDefault("device.retry_attempts", cfg.Device.RetryAttempts)
	viper.SetDefault("device.retry_delay", cfg.Device.RetryDelay)
	viper.SetDefault("test.max_concurrency", cfg.Test.MaxConcurrency)
	viper.SetDefault("test.cache_ttl", cfg.Test.CacheTTL)
	viper.SetDefault("test.cache_size", cfg.Test.CacheSize)
	viper.SetDefault("reporter.default_format", cfg.Reporter.DefaultFormat)
	viper.SetDefault("reporter.output_file", cfg.Reporter.OutputFile)

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLogLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}

	if c.Device.Timeout <= 0 {
		return fmt.Errorf("device timeout must be positive")
	}

	if c.Device.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	if c.Device.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts must be non-negative")
	}

	if c.Test.MaxConcurrency <= 0 {
		return fmt.Errorf("test max concurrency must be positive")
	}

	if c.Test.CacheSize < 0 {
		return fmt.Errorf("cache size must be non-negative")
	}

	validFormats := map[string]bool{
		"table":    true,
		"csv":      true,
		"json":     true,
		"markdown": true,
	}

	if !validFormats[c.Reporter.DefaultFormat] {
		return fmt.Errorf("invalid default format: %s", c.Reporter.DefaultFormat)
	}

	return nil
}

func GetConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		return &defaultConfig
	}
	return cfg
}