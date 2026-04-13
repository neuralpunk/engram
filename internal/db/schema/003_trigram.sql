-- 0.4.0: secondary FTS5 index using trigram tokenization for typo
-- tolerance and partial-match recall. Searched as a fallback tier
-- after the primary phrase/AND/OR cascade.

DROP TRIGGER IF EXISTS corrections_tri_ai;
DROP TRIGGER IF EXISTS corrections_tri_ad;
DROP TRIGGER IF EXISTS corrections_tri_au;
DROP TABLE IF EXISTS corrections_fts_tri;

CREATE VIRTUAL TABLE corrections_fts_tri USING fts5(
    fact,
    wrong,
    tags,
    trigger_hint,
    content='corrections',
    content_rowid='id',
    tokenize='trigram'
);

INSERT INTO corrections_fts_tri(corrections_fts_tri) VALUES('rebuild');

CREATE TRIGGER corrections_tri_ai AFTER INSERT ON corrections BEGIN
    INSERT INTO corrections_fts_tri(rowid, fact, wrong, tags, trigger_hint)
    VALUES (new.id, new.fact, new.wrong, new.tags, new.trigger_hint);
END;

CREATE TRIGGER corrections_tri_ad AFTER DELETE ON corrections BEGIN
    INSERT INTO corrections_fts_tri(corrections_fts_tri, rowid, fact, wrong, tags, trigger_hint)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags, old.trigger_hint);
END;

CREATE TRIGGER corrections_tri_au AFTER UPDATE ON corrections BEGIN
    INSERT INTO corrections_fts_tri(corrections_fts_tri, rowid, fact, wrong, tags, trigger_hint)
    VALUES ('delete', old.id, old.fact, old.wrong, old.tags, old.trigger_hint);
    INSERT INTO corrections_fts_tri(rowid, fact, wrong, tags, trigger_hint)
    VALUES (new.id, new.fact, new.wrong, new.tags, new.trigger_hint);
END;

INSERT INTO schema_version (version) VALUES (3);
