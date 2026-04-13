CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

INSERT INTO schema_version (version) VALUES (1);

CREATE TABLE corrections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    fact        TEXT NOT NULL,
    wrong       TEXT,
    scope       TEXT NOT NULL,
    tags        TEXT,
    source      TEXT,
    confidence  REAL DEFAULT 1.0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    hit_count   INTEGER DEFAULT 0,
    last_hit    INTEGER
);

CREATE VIRTUAL TABLE corrections_fts USING fts5(
    fact,
    wrong,
    tags,
    content='corrections',
    content_rowid='id',
    tokenize='porter ascii'
);

CREATE TRIGGER corrections_ai AFTER INSERT ON corrections BEGIN
    INSERT INTO corrections_fts(rowid, fact, wrong, tags)
    VALUES (new.id, new.fact, new.wrong, new.tags);
END;

CREATE TRIGGER corrections_ad AFTER DELETE ON corrections BEGIN
    INSERT INTO corrections_fts(corrections_fts, rowid, fact, wrong, tags)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags);
END;

CREATE TRIGGER corrections_au AFTER UPDATE ON corrections BEGIN
    INSERT INTO corrections_fts(corrections_fts, rowid, fact, wrong, tags)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags);
    INSERT INTO corrections_fts(rowid, fact, wrong, tags)
    VALUES (new.id, new.fact, new.wrong, new.tags);
END;

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    project     TEXT,
    created_at  INTEGER NOT NULL
);

CREATE TABLE injection_log (
    session_id      TEXT,
    correction_id   INTEGER,
    injected_at     INTEGER NOT NULL,
    token_estimate  INTEGER
);
