-- name: UpsertMessage :exec
INSERT INTO messages (telegram_msg_id, chat_id, chat_type, sender_id, sender_type, message_text, date, edit_date, is_outgoing, reply_to_msg_id, media_type, raw_json, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
ON CONFLICT(telegram_msg_id, chat_id, chat_type)
DO UPDATE SET
    message_text = excluded.message_text,
    edit_date = excluded.edit_date,
    raw_json = excluded.raw_json,
    media_type = excluded.media_type,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: AddChatFilter :exec
INSERT OR REPLACE INTO chat_filters (chat_id, chat_type, identifier, note) VALUES (?, ?, ?, ?);

-- name: RemoveChatFilter :exec
DELETE FROM chat_filters WHERE chat_id = ?;

-- name: ListChatFilters :many
SELECT * FROM chat_filters ORDER BY created_at;

-- name: IsChatFiltered :one
SELECT count(*) FROM chat_filters WHERE chat_id = ?;

-- name: GetAppState :one
SELECT value FROM app_state WHERE key = ?;

-- name: SetAppState :exec
INSERT OR REPLACE INTO app_state (key, value) VALUES (?, ?);

-- name: GetUpdateState :one
SELECT pts, qts, date, seq FROM update_state WHERE user_id = ?;

-- name: UpsertUpdateState :exec
INSERT OR REPLACE INTO update_state (user_id, pts, qts, date, seq) VALUES (?, ?, ?, ?, ?);

-- name: UpdatePts :execresult
UPDATE update_state SET pts = ? WHERE user_id = ?;

-- name: UpdateQts :execresult
UPDATE update_state SET qts = ? WHERE user_id = ?;

-- name: UpdateDate :execresult
UPDATE update_state SET date = ? WHERE user_id = ?;

-- name: UpdateSeq :execresult
UPDATE update_state SET seq = ? WHERE user_id = ?;

-- name: UpdateDateSeq :execresult
UPDATE update_state SET date = ?, seq = ? WHERE user_id = ?;

-- name: GetChannelPts :one
SELECT pts FROM channel_state WHERE user_id = ? AND channel_id = ?;

-- name: UpsertChannelPts :exec
INSERT OR REPLACE INTO channel_state (user_id, channel_id, pts) VALUES (?, ?, ?);

-- name: ListChannelStates :many
SELECT channel_id, pts FROM channel_state WHERE user_id = ?;

-- name: GetChannelAccessHash :one
SELECT access_hash FROM channel_access_hash WHERE user_id = ? AND channel_id = ?;

-- name: UpsertChannelAccessHash :exec
INSERT OR REPLACE INTO channel_access_hash (user_id, channel_id, access_hash) VALUES (?, ?, ?);
