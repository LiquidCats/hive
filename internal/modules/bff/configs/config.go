package configs

import (
	"time"

	"github.com/LiquidCats/hive/internal/kernel/configs"
)

type Config struct {
	DB   DBConfig        `yaml:"db" envconfig:"DB"`
	Auth AuthConfig      `yaml:"auth" envconfig:"AUTH"`
	Logs configs.Logging `yaml:"logs" envconfig:"LOGS"`
	Http configs.HTTP    `yaml:"http" envconfig:"HTTP"`
}

type AuthConfig struct {
	RPDisplayName string        `yaml:"rp_display_name" envconfig:"RP_DISPLAY_NAME"`
	RPID          string        `yaml:"rp_id" envconfig:"RP_ID"`
	RPOrigins     []string      `yaml:"rp_origins" envconfig:"RP_ORIGINS"`
	SessionTTL    time.Duration `default:"24h" yaml:"session_ttl" envconfig:"SESSION_TTL"`
}

type DBConfig struct {
	Path string `default:"hive.db" yaml:"path" envconfig:"PATH"`
}
