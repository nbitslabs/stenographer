package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"sync"

	"go.uber.org/zap"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/nbitslabs/stenographer/internal/config"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
)

//go:embed static/*
var staticFiles embed.FS

// ChatSummary represents a chat with its message stats and filter status.
type ChatSummary struct {
	ChatID           int64  `json:"chat_id"`
	ChatType         string `json:"chat_type"`
	Title            string `json:"title"`
	Username         string `json:"username"`
	MessageCount     int64  `json:"message_count"`
	LastMessageDate  int64  `json:"last_message_date"`
	LastMessage      string `json:"last_message"`
	IsWhitelisted    bool   `json:"is_whitelisted"`
	IsBlacklisted    bool   `json:"is_blacklisted"`
	IsTracked        bool   `json:"is_tracked"`
	FilterIdentifier string `json:"filter_identifier"`
}

// Server is the web UI HTTP server.
type Server struct {
	db      *sql.DB
	queries *sqlc.Queries
	cfg     *config.Config
	log     *zap.Logger

	resolveMu sync.Mutex // serialize resolve requests
}

// New creates a new web server.
func New(db *sql.DB, cfg *config.Config, log *zap.Logger) *Server {
	return &Server{
		db:      db,
		queries: sqlc.New(db),
		cfg:     cfg,
		log:     log,
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	// API routes.
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("GET /api/chats", s.handleListChats)
	mux.HandleFunc("POST /api/chats/filter", s.handleSetFilter)
	mux.HandleFunc("DELETE /api/chats/filter", s.handleRemoveFilter)
	mux.HandleFunc("POST /api/chats/resolve", s.handleResolveNames)

	// Static files.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	s.log.Info("starting web UI", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{
		"filter_mode": s.cfg.Filter.Mode,
	})
}

func (s *Server) handleListChats(w http.ResponseWriter, r *http.Request) {
	chats, err := s.listChats(r.Context())
	if err != nil {
		s.log.Error("list chats", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, chats)
}

func (s *Server) handleSetFilter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID     int64  `json:"chat_id"`
		ChatType   string `json:"chat_type"`
		FilterType string `json:"filter_type"` // "whitelist" or "blacklist"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.FilterType != "whitelist" && req.FilterType != "blacklist" {
		http.Error(w, "filter_type must be 'whitelist' or 'blacklist'", http.StatusBadRequest)
		return
	}

	// Remove existing filters for this chat first, then add the new one.
	if err := s.queries.RemoveChatFilterByID(r.Context(), req.ChatID); err != nil {
		s.log.Error("remove old filters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err := s.queries.AddChatFilter(r.Context(), sqlc.AddChatFilterParams{
		ChatID:     req.ChatID,
		ChatType:   req.ChatType,
		FilterType: req.FilterType,
		Identifier: strconv.FormatInt(req.ChatID, 10),
	})
	if err != nil {
		s.log.Error("add filter", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveFilter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID int64 `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.queries.RemoveChatFilterByID(r.Context(), req.ChatID); err != nil {
		s.log.Error("remove filter", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleResolveNames(w http.ResponseWriter, r *http.Request) {
	if !s.resolveMu.TryLock() {
		http.Error(w, "name resolution already in progress", http.StatusConflict)
		return
	}
	defer s.resolveMu.Unlock()

	s.log.Info("resolving chat names via Telegram API")

	resolved, err := s.resolveNames(r.Context())
	if err != nil {
		s.log.Error("resolve names", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]int{"resolved": resolved})
}

// resolveNames connects to Telegram, fetches names for all known chat IDs,
// and caches them in the chats table.
func (s *Server) resolveNames(ctx context.Context) (int, error) {
	// Collect distinct chat IDs and types from the messages table.
	type chatKey struct {
		id   int64
		typ  string
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT chat_id, chat_type FROM messages`)
	if err != nil {
		return 0, fmt.Errorf("query chat IDs: %w", err)
	}
	var keys []chatKey
	for rows.Next() {
		var k chatKey
		if err := rows.Scan(&k.id, &k.typ); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	rows.Close()
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

	// Look up channel access hashes we have stored.
	channelHashes := make(map[int64]int64)
	for _, cid := range channelIDs {
		hash, err := s.queries.GetChannelAccessHash(ctx, sqlc.GetChannelAccessHashParams{
			// Use user_id 0 as fallback — try all stored hashes.
			UserID:    0,
			ChannelID: cid,
		})
		if err == nil {
			channelHashes[cid] = hash
		}
	}
	// Also try with any user_id we can find.
	hashRows, err := s.db.QueryContext(ctx, `SELECT channel_id, access_hash FROM channel_access_hash`)
	if err == nil {
		for hashRows.Next() {
			var cid, hash int64
			if hashRows.Scan(&cid, &hash) == nil {
				channelHashes[cid] = hash
			}
		}
		hashRows.Close()
	}

	waiter := floodwait.NewSimpleWaiter().WithMaxRetries(3)
	client := telegram.NewClient(s.cfg.Telegram.AppID, s.cfg.Telegram.AppHash, telegram.Options{
		Logger:         s.log.Named("td"),
		SessionStorage: &session.FileStorage{Path: s.cfg.Telegram.SessionFile},
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
			inputs := make([]tg.InputUserClass, len(userIDs))
			for i, id := range userIDs {
				inputs[i] = &tg.InputUser{UserID: id}
			}
			users, err := api.UsersGetUsers(ctx, inputs)
			if err != nil {
				s.log.Warn("failed to resolve users", zap.Error(err))
			} else {
				for _, u := range users {
					if user, ok := u.(*tg.User); ok {
						title := user.FirstName
						if user.LastName != "" {
							title += " " + user.LastName
						}
						if err := s.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
							ChatID:   user.ID,
							ChatType: "user",
							Title:    title,
							Username: user.Username,
						}); err == nil {
							resolved++
						}
					}
				}
			}
		}

		// Resolve group chats.
		if len(chatIDs) > 0 {
			chats, err := api.MessagesGetChats(ctx, chatIDs)
			if err != nil {
				s.log.Warn("failed to resolve chats", zap.Error(err))
			} else {
				for _, c := range chats.GetChats() {
					if chat, ok := c.(*tg.Chat); ok {
						if err := s.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
							ChatID:   chat.ID,
							ChatType: "chat",
							Title:    chat.Title,
						}); err == nil {
							resolved++
						}
					}
				}
			}
		}

		// Resolve channels.
		if len(channelIDs) > 0 {
			var inputs []tg.InputChannelClass
			for _, cid := range channelIDs {
				hash, ok := channelHashes[cid]
				if !ok {
					continue
				}
				inputs = append(inputs, &tg.InputChannel{
					ChannelID:  cid,
					AccessHash: hash,
				})
			}
			if len(inputs) > 0 {
				chats, err := api.ChannelsGetChannels(ctx, inputs)
				if err != nil {
					s.log.Warn("failed to resolve channels", zap.Error(err))
				} else {
					for _, c := range chats.GetChats() {
						if ch, ok := c.(*tg.Channel); ok {
							if err := s.queries.UpsertChat(ctx, sqlc.UpsertChatParams{
								ChatID:   ch.ID,
								ChatType: "channel",
								Title:    ch.Title,
								Username: ch.Username,
							}); err == nil {
								resolved++
							}
						}
					}
				}
			}
		}

		return nil
	})

	return resolved, err
}

const listChatsQuery = `
SELECT
    m.chat_id,
    m.chat_type,
    COALESCE(c.title, '') AS title,
    COALESCE(c.username, '') AS username,
    COUNT(*) AS message_count,
    MAX(m.date) AS last_message_date,
    COALESCE(
        (SELECT m2.message_text FROM messages m2
         WHERE m2.chat_id = m.chat_id AND m2.chat_type = m.chat_type
           AND m2.message_text != ''
         ORDER BY m2.date DESC LIMIT 1),
        ''
    ) AS last_message,
    COALESCE(
        (SELECT cf.filter_type FROM chat_filters cf
         WHERE cf.chat_id = m.chat_id AND cf.filter_type = 'whitelist' LIMIT 1),
        ''
    ) AS whitelist_status,
    COALESCE(
        (SELECT cf.filter_type FROM chat_filters cf
         WHERE cf.chat_id = m.chat_id AND cf.filter_type = 'blacklist' LIMIT 1),
        ''
    ) AS blacklist_status,
    COALESCE(
        (SELECT cf.identifier FROM chat_filters cf
         WHERE cf.chat_id = m.chat_id LIMIT 1),
        ''
    ) AS filter_identifier
FROM messages m
LEFT JOIN chats c ON c.chat_id = m.chat_id AND c.chat_type = m.chat_type
GROUP BY m.chat_id, m.chat_type
ORDER BY last_message_date DESC
`

func (s *Server) listChats(ctx context.Context) ([]ChatSummary, error) {
	rows, err := s.db.QueryContext(ctx, listChatsQuery)
	if err != nil {
		return nil, fmt.Errorf("query chats: %w", err)
	}
	defer rows.Close()

	mode := s.cfg.Filter.Mode
	if mode != "allowlist_only" {
		mode = "default"
	}

	var chats []ChatSummary
	for rows.Next() {
		var c ChatSummary
		var whitelistStatus, blacklistStatus string
		if err := rows.Scan(
			&c.ChatID, &c.ChatType, &c.Title, &c.Username,
			&c.MessageCount, &c.LastMessageDate, &c.LastMessage,
			&whitelistStatus, &blacklistStatus,
			&c.FilterIdentifier,
		); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}

		c.IsWhitelisted = whitelistStatus == "whitelist"
		c.IsBlacklisted = blacklistStatus == "blacklist"

		// Compute tracking status based on filter mode.
		if mode == "allowlist_only" {
			c.IsTracked = c.IsWhitelisted
		} else {
			// Default mode: channels need whitelist, others tracked unless blacklisted.
			if c.ChatType == "channel" {
				c.IsTracked = c.IsWhitelisted
			} else {
				c.IsTracked = !c.IsBlacklisted
			}
		}

		// Truncate long preview messages.
		if len(c.LastMessage) > 200 {
			c.LastMessage = c.LastMessage[:200] + "..."
		}

		chats = append(chats, c)
	}
	if chats == nil {
		chats = []ChatSummary{}
	}
	return chats, rows.Err()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
