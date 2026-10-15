package configs

import "log/slog"

type Logging struct {
	AddSource bool       `default:"false" yaml:"add_source" envconfig:"ADD_SOURCE"`
	Level     slog.Level `default:"info" yaml:"level" envconfig:"LEVEL"`
}
