package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/nbitslabs/stenographer/internal/database"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
	"github.com/nbitslabs/stenographer/internal/telegram"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start logging Telegram messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		log, err := buildLogger(cfg.Logging.Level)
		if err != nil {
			return err
		}
		defer log.Sync()

		db, err := database.Open(cfg.Database.Path)
		if err != nil {
			return err
		}
		defer db.Close()

		queries := sqlc.New(db)

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		log.Info("starting stenographer",
			zap.String("db", cfg.Database.Path),
			zap.String("filter_mode", cfg.Filter.Mode),
		)

		return telegram.Run(ctx, cfg, queries, log)
	},
}

func buildLogger(level string) (*zap.Logger, error) {
	zcfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		zcfg.Level.SetLevel(zap.DebugLevel)
	case "warn":
		zcfg.Level.SetLevel(zap.WarnLevel)
	case "error":
		zcfg.Level.SetLevel(zap.ErrorLevel)
	default:
		zcfg.Level.SetLevel(zap.InfoLevel)
	}
	return zcfg.Build()
}

func init() {
	rootCmd.AddCommand(runCmd)
}
