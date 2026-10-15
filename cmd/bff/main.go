package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/LiquidCats/hive/internal/modules/bff"
	"github.com/LiquidCats/hive/internal/modules/bff/configs"
	"github.com/LiquidCats/libraries/cfg"
)

func main() {
	ctx := context.Background()

	config, err := cfg.Load[configs.Config]()
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: config.Logs.AddSource,
		Level:     config.Logs.Level,
	}))

	bff.Run(ctx, logger.WithGroup(bff.AppName), config)
}
