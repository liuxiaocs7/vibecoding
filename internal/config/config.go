package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Addr      string
	DataDir   string
	Open      bool
	LogLevel  string
	LogFormat string
	LogOutput string
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
	return cfg, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "vibecoding.db")
}
