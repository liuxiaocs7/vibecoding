package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr      string
	DataDir   string
	Open      bool
	LogLevel  string
	LogFormat string
	LogOutput string
	Token     string // required when Addr is not loopback
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vibecoding"
	}
	return filepath.Join(home, ".vibecoding")
}

func Parse() (*Config, error) {
	cfg := &Config{}
	flag.StringVar(&cfg.Addr, "addr", "127.0.0.1:8090", "HTTP listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", DefaultDataDir(), "data directory for SQLite and config")
	flag.BoolVar(&cfg.Open, "open", false, "open browser after start")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "log level: debug|info|warn|error")
	flag.StringVar(&cfg.LogFormat, "log-format", "text", "log format: text|json")
	flag.StringVar(&cfg.LogOutput, "log-output", "stdout", "log output: stdout|stderr|discard|/path/to/file.log")
	flag.StringVar(&cfg.Token, "token", "", "API token required when binding non-localhost (or set VIBECODING_TOKEN)")
	flag.Parse()

	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir()
	}
	abs, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data-dir: %w", err)
	}
	cfg.DataDir = abs
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data-dir: %w", err)
	}
	if strings.TrimSpace(cfg.Token) == "" {
		cfg.Token = strings.TrimSpace(os.Getenv("VIBECODING_TOKEN"))
	}
	if err := cfg.ValidateListenAuth(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ValidateListenAuth fails when binding a non-loopback address without a token.
func (c *Config) ValidateListenAuth() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if IsLoopbackAddr(c.Addr) {
		return nil
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("binding %s requires --token or VIBECODING_TOKEN (non-localhost API must be authenticated)", c.Addr)
	}
	return nil
}

// IsLoopbackAddr reports whether host in host:port (or bare host) is loopback.
func IsLoopbackAddr(addr string) bool {
	host := strings.TrimSpace(addr)
	if host == "" {
		return true
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0:0:0:0:0:0:0:1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "vibecoding.db")
}

// AuthRequired is true when a token is configured (non-loopback binds always set one).
func (c *Config) AuthRequired() bool {
	return c != nil && strings.TrimSpace(c.Token) != ""
}
