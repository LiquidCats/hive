package configs

import "time"

type HTTP struct {
	Port         string        `default:"8080" yaml:"port" envconfig:"PORT"`
	ReadTimeout  time.Duration `default:"5s" yaml:"read_timeout" envconfig:"READ_TIMEOUT"`
	WriteTimeout time.Duration `default:"10s" yaml:"write_timeout" envconfig:"WRITE_TIMEOUT"`
}
