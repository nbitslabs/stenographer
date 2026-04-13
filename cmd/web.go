package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/nbitslabs/stenographer/internal/database"
	"github.com/nbitslabs/stenographer/internal/web"
)

var webAddr string

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web UI for managing chats and filters",
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

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		srv := web.New(db, cfg, log)

		log.Info("web UI available",
			zap.String("addr", webAddr),
			zap.String("filter_mode", cfg.Filter.Mode),
		)

		return srv.ListenAndServe(ctx, webAddr)
	},
}

func init() {
	webCmd.Flags().StringVar(&webAddr, "addr", "127.0.0.1:8080", "listen address for the web UI")
	rootCmd.AddCommand(webCmd)
}
