package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Database DatabaseConfig `toml:"database"`
	Counter  CounterConfig  `toml:"counter"`
}

type ServerConfig struct {
	Addr         string        `toml:"addr"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
}

type DatabaseConfig struct {
	DSN string `toml:"dsn"`
}

type CounterConfig struct {
	SiteKey        string   `toml:"site_key"`
	AllowedOrigins []string `toml:"allowed_origins"` // 允许的域名列表
	EnableCors     bool     `toml:"enable_cors"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:         ":8080",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Database: DatabaseConfig{
			DSN: "data/counter.db",
		},
		Counter: CounterConfig{
			SiteKey:        "dtc_site",
			AllowedOrigins: []string{},
			EnableCors:     false,
		},
	}
}

// LoadConfig 从文件加载配置，环境变量覆盖
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	// 读取配置文件
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// 环境变量覆盖
	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("COUNTER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("COUNTER_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("COUNTER_SITE_KEY"); v != "" {
		cfg.Counter.SiteKey = v
	}
	if v := os.Getenv("COUNTER_ALLOWED_ORIGINS"); v != "" {
		cfg.Counter.AllowedOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("COUNTER_ENABLE_CORS"); v != "" {
		cfg.Counter.EnableCors = v == "true" || v == "1"
	}
	if v := os.Getenv("COUNTER_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadTimeout = d
		}
	}
	if v := os.Getenv("COUNTER_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.WriteTimeout = d
		}
	}
}

// WriteConfig 将当前配置写入文件
func WriteConfig(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// HasOrigins 是否配置了域名白名单
func (c *CounterConfig) HasOrigins() bool {
	return len(c.AllowedOrigins) > 0
}

// IsOriginAllowed 检查 origin 是否在白名单中
func (c *CounterConfig) IsOriginAllowed(origin string) bool {
	if !c.HasOrigins() {
		return true // 未配置白名单，放行所有
	}
	host, ok := originHostname(origin)
	if !ok {
		return false
	}
	for _, allowed := range c.AllowedOrigins {
		allowedHost, ok := configuredHostname(allowed)
		if ok && strings.EqualFold(host, allowedHost) {
			return true
		}
	}
	return false
}

func originHostname(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return strings.ToLower(parsed.Hostname()), true
}

func configuredHostname(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if !strings.Contains(raw, "://") {
		raw = "//" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return "", false
	}
	return strings.ToLower(parsed.Hostname()), true
}
