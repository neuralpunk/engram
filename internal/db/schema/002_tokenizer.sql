-- 0.4.0: rebuild FTS5 with a code-friendly tokenizer.
-- unicode61 handles non-ASCII and case folding properly.
-- remove_diacritics 1 lets "cafe" match "café".
-- tokenchars '-_./' keeps identifiers like burntsushi/toml,
-- use_state, grpc-web, config.toml as single tokens.

DROP TRIGGER IF EXISTS corrections_ai;
DROP TRIGGER IF EXISTS corrections_ad;
DROP TRIGGER IF EXISTS corrections_au;

DROP TABLE IF EXISTS corrections_fts;

CREATE VIRTUAL TABLE corrections_fts USING fts5(
    fact,
    wrong,
    tags,
    trigger_hint,
    content='corrections',
    content_rowid='id',
    tokenize="unicode61 remove_diacritics 1 tokenchars '-_./'"
);

INSERT INTO corrections_fts(corrections_fts) VALUES('rebuild');

CREATE TRIGGER corrections_ai AFTER INSERT ON corrections BEGIN
    INSERT INTO corrections_fts(rowid, fact, wrong, tags, trigger_hint)
    VALUES (new.id, new.fact, new.wrong, new.tags, new.trigger_hint);
END;

CREATE TRIGGER corrections_ad AFTER DELETE ON corrections BEGIN
    INSERT INTO corrections_fts(corrections_fts, rowid, fact, wrong, tags, trigger_hint)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags, old.trigger_hint);
END;

CREATE TRIGGER corrections_au AFTER UPDATE ON corrections BEGIN
    INSERT INTO corrections_fts(corrections_fts, rowid, fact, wrong, tags, trigger_hint)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags, old.trigger_hint);
    INSERT INTO corrections_fts(rowid, fact, wrong, tags, trigger_hint)
    VALUES (new.id, new.fact, new.wrong, new.tags, new.trigger_hint);
END;

INSERT INTO schema_version (version) VALUES (2);
