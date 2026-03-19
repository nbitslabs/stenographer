package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/nbitslabs/stenographer/internal/database"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
	"github.com/nbitslabs/stenographer/internal/telegram"
)

var blacklistCmd = &cobra.Command{
	Use:   "blacklist",
	Short: "Manage the chat blacklist",
}

var blacklistAddCmd = &cobra.Command{
	Use:   "add <identifier>...",
	Short: "Add chat(s) to the blacklist",
	Long:  "Add by chat ID (numeric), @username, or t.me link",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return addFilter(args, "blacklist")
	},
}

var blacklistRemoveCmd = &cobra.Command{
	Use:   "remove <identifier>...",
	Short: "Remove chat(s) from the blacklist",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return removeFilter(args, "blacklist")
	},
}

var blacklistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all blacklisted chats",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listFilters("blacklist")
	},
}

func init() {
	rootCmd.AddCommand(blacklistCmd)
	blacklistCmd.AddCommand(blacklistAddCmd)
	blacklistCmd.AddCommand(blacklistRemoveCmd)
	blacklistCmd.AddCommand(blacklistListCmd)
}

func addFilter(identifiers []string, filterType string) error {
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	queries := sqlc.New(db)

	for _, ident := range identifiers {
		chatID, chatType, originalIdent, err := resolveIdentifier(ident)
		if err != nil {
			fmt.Printf("error resolving %q: %v\n", ident, err)
			continue
		}

		err = queries.AddChatFilter(context.Background(), sqlc.AddChatFilterParams{
			ChatID:     chatID,
			ChatType:   chatType,
			FilterType: filterType,
			Identifier: originalIdent,
		})
		if err != nil {
			fmt.Printf("error adding %q: %v\n", ident, err)
			continue
		}

		fmt.Printf("added %s (chat_id=%d, type=%s, filter=%s)\n", originalIdent, chatID, chatType, filterType)
	}

	return nil
}

func removeFilter(identifiers []string, filterType string) error {
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	queries := sqlc.New(db)

	for _, ident := range identifiers {
		chatID, _, _, err := resolveIdentifier(ident)
		if err != nil {
			fmt.Printf("error resolving %q: %v\n", ident, err)
			continue
		}

		err = queries.RemoveChatFilter(context.Background(), sqlc.RemoveChatFilterParams{
			ChatID:     chatID,
			FilterType: filterType,
		})
		if err != nil {
			fmt.Printf("error removing %q: %v\n", ident, err)
			continue
		}

		fmt.Printf("removed chat_id=%d from %s\n", chatID, filterType)
	}

	return nil
}

func listFilters(filterType string) error {
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	queries := sqlc.New(db)

	filters, err := queries.ListChatFiltersByType(context.Background(), filterType)
	if err != nil {
		return err
	}

	if len(filters) == 0 {
		fmt.Println("no entries")
		return nil
	}

	for _, f := range filters {
		ident := f.Identifier
		if ident == "" {
			ident = strconv.FormatInt(f.ChatID, 10)
		}
		fmt.Printf("  %s  chat_id=%d  type=%s  added=%s\n", ident, f.ChatID, f.ChatType, f.CreatedAt)
	}

	return nil
}

func resolveIdentifier(ident string) (chatID int64, chatType, original string, err error) {
	original = ident

	// Try numeric chat ID first.
	if id, parseErr := strconv.ParseInt(ident, 10, 64); parseErr == nil {
		return id, "", ident, nil
	}

	// Parse t.me link.
	username := ""
	if strings.HasPrefix(ident, "https://t.me/") || strings.HasPrefix(ident, "http://t.me/") {
		u, parseErr := url.Parse(ident)
		if parseErr != nil {
			return 0, "", "", fmt.Errorf("invalid URL: %w", parseErr)
		}
		username = strings.TrimPrefix(u.Path, "/")
	} else {
		username = strings.TrimPrefix(ident, "@")
	}

	if username == "" {
		return 0, "", "", fmt.Errorf("empty username from identifier %q", ident)
	}

	// Resolve via Telegram API.
	log, _ := zap.NewProduction()
	defer log.Sync()

	chatID, chatType, err = telegram.ResolveUsername(context.Background(), cfg, username, log)
	if err != nil {
		return 0, "", "", fmt.Errorf("resolve %q: %w", username, err)
	}

	return chatID, chatType, original, nil
}
