package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nbitslabs/stenographer/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "stenographer",
	Short: "Telegram message logger",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for help commands.
		if cmd.Name() == "help" {
			return nil
		}
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config %s: %w", cfgFile, err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.toml", "config file path")
}
