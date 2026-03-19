-- +goose Up

-- Add filter_type column to chat_filters.
ALTER TABLE chat_filters ADD COLUMN filter_type TEXT NOT NULL DEFAULT 'blacklist';

-- Drop old unique index and create new one on (chat_id, chat_type, filter_type).
-- SQLite doesn't support DROP CONSTRAINT, so we recreate the table.
CREATE TABLE chat_filters_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id     INTEGER NOT NULL,
    chat_type   TEXT    NOT NULL DEFAULT '',
    filter_type TEXT    NOT NULL DEFAULT 'blacklist',
    identifier  TEXT    NOT NULL DEFAULT '',
    note        TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(chat_id, chat_type, filter_type)
);

INSERT INTO chat_filters_new (id, chat_id, chat_type, filter_type, identifier, note, created_at)
SELECT id, chat_id, chat_type, filter_type, identifier, note, created_at
FROM chat_filters;

DROP TABLE chat_filters;
ALTER TABLE chat_filters_new RENAME TO chat_filters;

-- +goose Down

CREATE TABLE chat_filters_old (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id    INTEGER NOT NULL,
    chat_type  TEXT    NOT NULL DEFAULT '',
    identifier TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE(chat_id)
);

INSERT INTO chat_filters_old (id, chat_id, chat_type, identifier, note, created_at)
SELECT id, chat_id, chat_type, identifier, note, created_at
FROM chat_filters;

DROP TABLE chat_filters;
ALTER TABLE chat_filters_old RENAME TO chat_filters;
