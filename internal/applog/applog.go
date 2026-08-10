package applog

import (
	"fmt"
	"strings"

	goconfig "github.com/ymhhh/go-common/config"
	"github.com/ymhhh/go-common/logger"
)

// Init configures the global logger (github.com/ymhhh/go-common/logger).
// output: stdout | stderr | discard | /path/to/file.log
func Init(level, format, output string) error {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "text"
	}
	output = strings.TrimSpace(output)
	if output == "" {
		output = "stdout"
	}

	opts := goconfig.Options{
		"logger": map[string]any{
			"level":        level,
			"format":       format,
			"output":       output,
			"reportCaller": false,
			"text": map[string]any{
				"disableColors": false,
				"fullTimestamp": true,
			},
			"json": map[string]any{
				"prettyPrint": false,
			},
		},
	}
	if err := logger.InitGlobal(opts.ToConfig()); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	return nil
}
