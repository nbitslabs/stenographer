// Package names resolves Telegram chat/user IDs to display names,
// caching results in the chats table for fast lookups.
package names

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/zap"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/nbitslabs/stenographer/internal/config"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

// Resolver resolves and caches chat/user names.
type Resolver struct {
	db      *sql.DB
	queries *sqlc.Queries
	cfg     *config.Config
	log     *zap.Logger
}

// New creates a new name resolver.
func New(db *sql.DB, cfg *config.Config, log *zap.Logger) *Resolver {
	return &Resolver{
		db:      db,
		queries: sqlc.New(db),
		cfg:     cfg,
		log:     log,
	}
}

// CachedNames returns a map of ID -> display name from the chats table.
// This does not hit the Telegram API.
func (r *Resolver) CachedNames(ctx context.Context) (map[int64]string, error) {
	chats, err := r.queries.ListChats(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(chats))
	for _, c := range chats {
		name := displayName(c.Title, c.Username)
		if name != "" {
			names[c.ChatID] = name
		}
	}
	return names, nil
}

// ResolveAll connects to Telegram and resolves names for all known chats
// (from both the chats table and messages table). Returns the number of
// newly resolved names.
func (r *Resolver) ResolveAll(ctx context.Context) (int, error) {
	keys, err := r.allChatKeys(ctx)
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}

	// Partition by type.
	var userIDs, chatIDs, channelIDs []int64
	for _, k := range keys {
		switch k.typ {
		case "user":
			userIDs = append(userIDs, k.id)
		case "chat":
			chatIDs = append(chatIDs, k.id)
		case "channel":
			channelIDs = append(channelIDs, k.id)
		}
	}

	channelHashes, err := r.loadChannelHashes(ctx)
	if err != nil {
		r.log.Warn("failed to load channel hashes", zap.Error(err))
	}

	waiter := floodwait.NewSimpleWaiter().WithMaxRetries(3)
	client := telegram.NewClient(r.cfg.Telegram.AppID, r.cfg.Telegram.AppHash, telegram.Options{
		Logger:         r.log.Named("td"),
		SessionStorage: &session.FileStorage{Path: r.cfg.Telegram.SessionFile},
		Middlewares:    []telegram.Middleware{waiter},
	})

	resolved := 0
	err = client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("not authenticated, run 'stenographer run' first to log in")
		}

		api := client.API()

		// Resolve users.
		if len(userIDs) > 0 {
			n, err := r.resolveUsers(ctx, api, userIDs)
			if err != nil {
				r.log.Warn("partial user resolution failure", zap.Error(err))
			}
			resolved += n
		}

		// Resolve group chats.
		if len(chatIDs) > 0 {
			n, err := r.resolveChats(ctx, api, chatIDs)
			if err != nil {
				r.log.Warn("partial chat resolution failure", zap.Error(err))
			}
			resolved += n
		}

		// Resolve channels/supergroups.
		if len(channelIDs) > 0 {
			n, err := r.resolveChannels(ctx, api, channelIDs, channelHashes)
			if err != nil {
				r.log.Warn("partial channel resolution failure", zap.Error(err))
			}
			resolved += n
		}

		return nil
	})

	return resolved, err
}

type chatKey struct {
	id  int64
	typ string
}

func (r *Resolver) allChatKeys(ctx context.Context) ([]chatKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT chat_id, chat_type FROM chats
		UNION
		SELECT DISTINCT chat_id, chat_type FROM messages
		UNION
		SELECT DISTINCT sender_id AS chat_id, 'user' AS chat_type FROM messages WHERE sender_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("query chat keys: %w", err)
	}
	defer rows.Close()

	var keys []chatKey
	for rows.Next() {
		var k chatKey
		if err := rows.Scan(&k.id, &k.typ); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *Resolver) loadChannelHashes(ctx context.Context) (map[int64]int64, error) {
	hashes := make(map[int64]int64)
	rows, err := r.db.QueryContext(ctx, `SELECT channel_id, access_hash FROM channel_access_hash`)
	if err != nil {
		return hashes, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, hash int64
		if rows.Scan(&cid, &hash) == nil {
			hashes[cid] = hash
		}
	}
	return hashes, rows.Err()
}

func (r *Resolver) resolveUsers(ctx context.Context, api *tg.Client, ids []int64) (int, error) {
	inputs := make([]tg.InputUserClass, len(ids))
	for i, id := range ids {
		inputs[i] = &tg.InputUser{UserID: id}
	}
	users, err := api.UsersGetUsers(ctx, inputs)
	if err != nil {
		return 0, err
	}
	resolved := 0
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			title := user.FirstName
			if user.LastName != "" {
				title += " " + user.LastName
			}
			if err := r.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
				ChatID:   user.ID,
				ChatType: "user",
				Title:    title,
				Username: user.Username,
			}); err == nil {
				resolved++
			}
		}
	}
	return resolved, nil
}

func (r *Resolver) resolveChats(ctx context.Context, api *tg.Client, ids []int64) (int, error) {
	chats, err := api.MessagesGetChats(ctx, ids)
	if err != nil {
		return 0, err
	}
	resolved := 0
	for _, c := range chats.GetChats() {
		if chat, ok := c.(*tg.Chat); ok {
			if err := r.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
				ChatID:   chat.ID,
				ChatType: "chat",
				Title:    chat.Title,
			}); err == nil {
				resolved++
			}
		}
	}
	return resolved, nil
}

func (r *Resolver) resolveChannels(ctx context.Context, api *tg.Client, ids []int64, hashes map[int64]int64) (int, error) {
	var inputs []tg.InputChannelClass
	for _, cid := range ids {
		hash, ok := hashes[cid]
		if !ok {
			continue
		}
		inputs = append(inputs, &tg.InputChannel{
			ChannelID:  cid,
			AccessHash: hash,
		})
	}
	if len(inputs) == 0 {
		return 0, nil
	}

	chats, err := api.ChannelsGetChannels(ctx, inputs)
	if err != nil {
		return 0, err
	}
	resolved := 0
	for _, c := range chats.GetChats() {
		if ch, ok := c.(*tg.Channel); ok {
			if err := r.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
				ChatID:   ch.ID,
				ChatType: "channel",
				Title:    ch.Title,
				Username: ch.Username,
			}); err == nil {
				resolved++
			}
		}
	}
	return resolved, nil
}

func displayName(title, username string) string {
	if title != "" && username != "" {
		return title + " (@" + username + ")"
	}
	if title != "" {
		return title
	}
	if username != "" {
		return "@" + username
	}
	return ""
}
