package query

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/nbitslabs/stenographer/internal/config"
)

// ResolveNames connects to Telegram and resolves chat/sender IDs to display names.
// Returns a map from ID -> name. IDs that cannot be resolved are omitted.
func ResolveNames(ctx context.Context, cfg *config.Config, ids []int64, log *zap.Logger) (map[int64]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	waiter := floodwait.NewSimpleWaiter().WithMaxRetries(3)

	client := telegram.NewClient(cfg.Telegram.AppID, cfg.Telegram.AppHash, telegram.Options{
		Logger:         log.Named("td"),
		SessionStorage: &session.FileStorage{Path: cfg.Telegram.SessionFile},
		Middlewares: []telegram.Middleware{
			waiter,
		},
	})

	names := make(map[int64]string)

	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("not authenticated, run 'stenographer run' first to log in")
		}

		api := client.API()

		// Try to resolve as users.
		userInputs := make([]tg.InputUserClass, 0, len(ids))
		for _, id := range ids {
			userInputs = append(userInputs, &tg.InputUser{UserID: id})
		}

		users, err := api.UsersGetUsers(ctx, userInputs)
		if err == nil {
			for _, u := range users {
				if user, ok := u.(*tg.User); ok {
					name := user.FirstName
					if user.LastName != "" {
						name += " " + user.LastName
					}
					if user.Username != "" {
						name = "@" + user.Username
					}
					if name != "" {
						names[user.ID] = name
					}
				}
			}
		}

		// Try to resolve remaining IDs as chats.
		var unresolvedChatIDs []int64
		for _, id := range ids {
			if _, ok := names[id]; !ok {
				unresolvedChatIDs = append(unresolvedChatIDs, id)
			}
		}

		if len(unresolvedChatIDs) > 0 {
			chatInputs := make([]int64, len(unresolvedChatIDs))
			copy(chatInputs, unresolvedChatIDs)
			chats, err := api.MessagesGetChats(ctx, chatInputs)
			if err == nil {
				for _, c := range chats.GetChats() {
					if chat, ok := c.(*tg.Chat); ok {
						names[chat.ID] = chat.Title
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Warn("name resolution partially failed", zap.Error(err))
	}

	return names, nil
}

// CollectIDs extracts unique chat_id and sender_id values from records.
func CollectIDs(records []map[string]any) []int64 {
	seen := make(map[int64]bool)
	for _, r := range records {
		if id, ok := toInt64(r["chat_id"]); ok && id != 0 {
			seen[id] = true
		}
		if id, ok := toInt64(r["sender_id"]); ok && id != 0 {
			seen[id] = true
		}
	}
	ids := make([]int64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}
