package telegram

import (
	"context"
	"database/sql"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/gotd/td/tg"
	"github.com/nbitslabs/stenographer/internal/database/sqlc"
	"github.com/nbitslabs/stenographer/internal/filter"
)

type MessageHandler struct {
	queries *sqlc.Queries
	filter  *filter.Checker
	log     *zap.Logger
}

func NewMessageHandler(queries *sqlc.Queries, f *filter.Checker, log *zap.Logger) *MessageHandler {
	return &MessageHandler{
		queries: queries,
		filter:  f,
		log:     log,
	}
}

func (h *MessageHandler) HandleNewMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
	return h.processMessage(ctx, e, update.Message)
}

func (h *MessageHandler) HandleNewChannelMessage(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
	return h.processMessage(ctx, e, update.Message)
}

func (h *MessageHandler) HandleEditMessage(ctx context.Context, e tg.Entities, update *tg.UpdateEditMessage) error {
	return h.processMessage(ctx, e, update.Message)
}

func (h *MessageHandler) HandleEditChannelMessage(ctx context.Context, e tg.Entities, update *tg.UpdateEditChannelMessage) error {
	return h.processMessage(ctx, e, update.Message)
}

func (h *MessageHandler) processMessage(ctx context.Context, e tg.Entities, msgClass tg.MessageClass) error {
	msg, ok := msgClass.(*tg.Message)
	if !ok {
		return nil // ignore MessageEmpty, MessageService
	}

	chatID, chatType := extractPeer(msg.PeerID)

	ok, err := h.filter.ShouldLog(ctx, chatID, chatType)
	if err != nil {
		h.log.Error("filter check failed", zap.Error(err))
		return nil // don't block on filter errors
	}
	if !ok {
		return nil
	}

	senderID, senderType := extractFromID(msg.FromID)

	rawJSON, _ := json.Marshal(msg)

	params := sqlc.UpsertMessageParams{
		TelegramMsgID: int64(msg.ID),
		ChatID:        chatID,
		ChatType:      chatType,
		SenderID:      toNullInt64(senderID),
		SenderType:    toNullString(senderType),
		MessageText:   msg.Message,
		Date:          int64(msg.Date),
		EditDate:      toNullInt64FromInt(msg.EditDate),
		IsOutgoing:    boolToInt64(msg.Out),
		ReplyToMsgID:  extractReplyTo(msg.ReplyTo),
		MediaType:     toNullString(mediaTypeName(msg.Media)),
		RawJson:       sql.NullString{String: string(rawJSON), Valid: true},
	}

	if err := h.queries.UpsertMessage(ctx, params); err != nil {
		h.log.Error("failed to save message",
			zap.Int("msg_id", msg.ID),
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)
		return nil // don't fail the update pipeline
	}

	h.log.Debug("saved message",
		zap.Int("msg_id", msg.ID),
		zap.Int64("chat_id", chatID),
		zap.String("chat_type", chatType),
	)

	return nil
}

func extractPeer(peer tg.PeerClass) (int64, string) {
	if peer == nil {
		return 0, ""
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		return p.UserID, "user"
	case *tg.PeerChat:
		return p.ChatID, "chat"
	case *tg.PeerChannel:
		return p.ChannelID, "channel"
	default:
		return 0, ""
	}
}

func extractFromID(peer tg.PeerClass) (int64, string) {
	if peer == nil {
		return 0, ""
	}
	return extractPeer(peer)
}

func extractReplyTo(replyTo tg.MessageReplyHeaderClass) sql.NullInt64 {
	if replyTo == nil {
		return sql.NullInt64{}
	}
	if r, ok := replyTo.(*tg.MessageReplyHeader); ok {
		if id, ok := r.GetReplyToMsgID(); ok {
			return sql.NullInt64{Int64: int64(id), Valid: true}
		}
	}
	return sql.NullInt64{}
}

func mediaTypeName(media tg.MessageMediaClass) string {
	if media == nil {
		return ""
	}
	switch media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	case *tg.MessageMediaGeo:
		return "geo"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaVenue:
		return "venue"
	case *tg.MessageMediaGame:
		return "game"
	case *tg.MessageMediaInvoice:
		return "invoice"
	case *tg.MessageMediaGeoLive:
		return "geo_live"
	case *tg.MessageMediaPoll:
		return "poll"
	case *tg.MessageMediaDice:
		return "dice"
	case *tg.MessageMediaWebPage:
		return "webpage"
	case *tg.MessageMediaStory:
		return "story"
	default:
		return "other"
	}
}

func toNullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func toNullInt64FromInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

func toNullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
