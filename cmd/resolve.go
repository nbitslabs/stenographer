package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/nbitslabs/stenographer/internal/database"
	"github.com/nbitslabs/stenographer/internal/names"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve and cache display names for all known chats and contacts",
	Long: `Connects to Telegram and resolves names for all known chat IDs (users,
groups, channels) and sender IDs. Results are cached in the database so
that subsequent queries with --resolve-names work instantly without
hitting the Telegram API.

Run this after initial setup and periodically to keep names fresh.`,
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

		resolver := names.New(db, cfg, log)

		fmt.Println("Resolving names via Telegram API...")
		resolved, err := resolver.ResolveAll(context.Background())
		if err != nil {
			log.Warn("resolution completed with errors", zap.Error(err))
		}
		fmt.Printf("Resolved %d names\n", resolved)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resolveCmd)
}
