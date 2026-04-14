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

	"github.com/nbitslabs/stenographer/internal/config"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
	"github.com/nbitslabs/stenographer/internal/names"
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

func (s *Server) resolveNames(ctx context.Context) (int, error) {
	resolver := names.New(s.db, s.cfg, s.log)
	return resolver.ResolveAll(ctx)
}

const listChatsQuery = `
WITH all_chats AS (
    SELECT chat_id, chat_type FROM chats
    UNION
    SELECT DISTINCT chat_id, chat_type FROM messages
)
SELECT
    ac.chat_id,
    ac.chat_type,
    COALESCE(c.title, '') AS title,
    COALESCE(c.username, '') AS username,
    COALESCE(msg.message_count, 0) AS message_count,
    COALESCE(msg.last_message_date, 0) AS last_message_date,
    COALESCE(
        (SELECT m2.message_text FROM messages m2
         WHERE m2.chat_id = ac.chat_id AND m2.chat_type = ac.chat_type
           AND m2.message_text != ''
         ORDER BY m2.date DESC LIMIT 1),
        ''
    ) AS last_message,
    COALESCE(
        (SELECT cf.filter_type FROM chat_filters cf
         WHERE cf.chat_id = ac.chat_id AND cf.filter_type = 'whitelist' LIMIT 1),
        ''
    ) AS whitelist_status,
    COALESCE(
        (SELECT cf.filter_type FROM chat_filters cf
         WHERE cf.chat_id = ac.chat_id AND cf.filter_type = 'blacklist' LIMIT 1),
        ''
    ) AS blacklist_status,
    COALESCE(
        (SELECT cf.identifier FROM chat_filters cf
         WHERE cf.chat_id = ac.chat_id LIMIT 1),
        ''
    ) AS filter_identifier
FROM all_chats ac
LEFT JOIN chats c ON c.chat_id = ac.chat_id AND c.chat_type = ac.chat_type
LEFT JOIN (
    SELECT chat_id, chat_type, COUNT(*) AS message_count, MAX(date) AS last_message_date
    FROM messages
    GROUP BY chat_id, chat_type
) msg ON msg.chat_id = ac.chat_id AND msg.chat_type = ac.chat_type
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
