-- +goose Up

CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_msg_id INTEGER NOT NULL,
    chat_id         INTEGER NOT NULL,
    chat_type       TEXT    NOT NULL DEFAULT '',
    sender_id       INTEGER,
    sender_type     TEXT,
    message_text    TEXT    NOT NULL DEFAULT '',
    date            INTEGER NOT NULL,
    edit_date       INTEGER,
    is_outgoing     INTEGER NOT NULL DEFAULT 0,
    reply_to_msg_id INTEGER,
    media_type      TEXT,
    raw_json        TEXT,
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(telegram_msg_id, chat_id, chat_type)
);

CREATE INDEX idx_messages_chat ON messages(chat_id, chat_type);
CREATE INDEX idx_messages_date ON messages(date);
CREATE INDEX idx_messages_sender ON messages(sender_id);

CREATE TABLE IF NOT EXISTS chat_filters (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id    INTEGER NOT NULL,
    chat_type  TEXT    NOT NULL DEFAULT '',
    identifier TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(chat_id)
);

CREATE TABLE IF NOT EXISTS update_state (
    user_id INTEGER NOT NULL,
    pts     INTEGER NOT NULL DEFAULT 0,
    qts     INTEGER NOT NULL DEFAULT 0,
    date    INTEGER NOT NULL DEFAULT 0,
    seq     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id)
);

CREATE TABLE IF NOT EXISTS channel_state (
    user_id    INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    pts        INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS channel_access_hash (
    user_id     INTEGER NOT NULL,
    channel_id  INTEGER NOT NULL,
    access_hash INTEGER NOT NULL,
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS app_state (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_filters;
DROP TABLE IF EXISTS update_state;
DROP TABLE IF EXISTS channel_state;
DROP TABLE IF EXISTS channel_access_hash;
DROP TABLE IF EXISTS app_state;
