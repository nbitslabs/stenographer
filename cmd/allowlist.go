package cmd

import (
	"github.com/spf13/cobra"
)

var allowlistCmd = &cobra.Command{
	Use:     "allowlist",
	Aliases: []string{"whitelist"},
	Short:   "Manage the chat allowlist (whitelist)",
}

var allowlistAddCmd = &cobra.Command{
	Use:   "add <identifier>...",
	Short: "Add chat(s) to the allowlist",
	Long:  "Add by chat ID (numeric), @username, or t.me link",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addFilter(args, "whitelist")
	},
}

var allowlistRemoveCmd = &cobra.Command{
	Use:   "remove <identifier>...",
	Short: "Remove chat(s) from the allowlist",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return removeFilter(args, "whitelist")
	},
}

var allowlistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all allowlisted chats",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listFilters("whitelist")
	},
}

func init() {
	rootCmd.AddCommand(allowlistCmd)
	allowlistCmd.AddCommand(allowlistAddCmd)
	allowlistCmd.AddCommand(allowlistRemoveCmd)
	allowlistCmd.AddCommand(allowlistListCmd)
}
