package telegram

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	updhook "github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"

	"github.com/nbitslabs/stenographer/internal/config"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
	"github.com/nbitslabs/stenographer/internal/filter"
)

func Run(ctx context.Context, cfg *config.Config, queries *sqlc.Queries, log *zap.Logger) error {
	dispatcher := tg.NewUpdateDispatcher()

	stateStorage := NewSQLiteStateStorage(queries)
	accessHasher := NewSQLiteAccessHasher(queries)

	gaps := updates.New(updates.Config{
		Handler:      dispatcher,
		Storage:      stateStorage,
		AccessHasher: accessHasher,
		Logger:       log.Named("gaps"),
	})

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		log.Warn("flood wait", zap.Duration("duration", wait.Duration))
	})

	client := telegram.NewClient(cfg.Telegram.AppID, cfg.Telegram.AppHash, telegram.Options{
		Logger:         log.Named("td"),
		SessionStorage: &session.FileStorage{Path: cfg.Telegram.SessionFile},
		UpdateHandler:  gaps,
		Middlewares: []telegram.Middleware{
			updhook.UpdateHook(gaps.Handle),
			waiter,
			ratelimit.New(rate.Every(100*time.Millisecond), 5),
		},
	})

	filterChecker := filter.New(queries, cfg.Filter.Mode)
	msgHandler := NewMessageHandler(queries, filterChecker, log)

	dispatcher.OnNewMessage(msgHandler.HandleNewMessage)
	dispatcher.OnNewChannelMessage(msgHandler.HandleNewChannelMessage)
	dispatcher.OnEditMessage(msgHandler.HandleEditMessage)
	dispatcher.OnEditChannelMessage(msgHandler.HandleEditChannelMessage)

	flow := auth.NewFlow(
		NewTerminalAuth(cfg.Telegram.Phone),
		auth.SendCodeOptions{},
	)

	return waiter.Run(ctx, func(ctx context.Context) error {
		return client.Run(ctx, func(ctx context.Context) error {
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return err
			}

			self, err := client.Self(ctx)
			if err != nil {
				return err
			}

			log.Info("authenticated", zap.String("username", self.Username), zap.Int64("id", self.ID))

			return gaps.Run(ctx, client.API(), self.ID, updates.AuthOptions{
				OnStart: func(ctx context.Context) {
					log.Info("listening for messages")
				},
			})
		})
	})
}

func ResolveUsername(ctx context.Context, cfg *config.Config, username string, log *zap.Logger) (int64, string, error) {
	waiter := floodwait.NewSimpleWaiter().WithMaxRetries(3)

	client := telegram.NewClient(cfg.Telegram.AppID, cfg.Telegram.AppHash, telegram.Options{
		Logger:         log.Named("td"),
		SessionStorage: &session.FileStorage{Path: cfg.Telegram.SessionFile},
		Middlewares: []telegram.Middleware{
			waiter,
		},
	})

	var chatID int64
	var chatType string

	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("not authenticated, run 'stenographer run' first to log in")
		}

		resolved, err := client.API().ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return err
		}

		chatID, chatType = extractPeer(resolved.Peer)
		return nil
	})

	return chatID, chatType, err
}
