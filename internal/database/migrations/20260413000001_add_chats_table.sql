-- +goose Up

CREATE TABLE IF NOT EXISTS chats (
    chat_id    INTEGER NOT NULL,
    chat_type  TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    username   TEXT    NOT NULL DEFAULT '',
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (chat_id, chat_type)
);

-- +goose Down
DROP TABLE IF EXISTS chats;
