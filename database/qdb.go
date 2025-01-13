// /home/krylon/go/src/github.com/blicero/snoopy/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-13 15:20:32 krylon>

package database

import "github.com/blicero/snoopy/database/query"

var dbQueries = map[query.ID]string{
	query.RootAdd:       "INSERT INTO root (path) VALUES (?) RETURNING id",
	query.RootGetByPath: "SELECT id, last_scan FROM root WHERE path = ?",
	query.RootGetByID:   "SELECT path, last_scan FROM root WHERE id = ?",
	query.RootGetAll:    "SELECT id, path, last_scan FROM root",
	query.RootDelete:    "DELETE FROM root WHERE id = ?",
	query.RootMarkScan:  "UPDATE root SET last_scan = ? WHERE id = ?",
	query.FileAdd: `
INSERT INTO file (root_id, path, mime_type, ctime)
VALUES           (      ?,    ?,         ?,     ?)
RETURNING id
`,
	query.FileDelete:      "DELETE FROM file WHERE id = ?",
	query.FileUpdateCtime: "UPDATE file SET ctime = ? WHERE id = ?",
	query.FileGetByPath: `
SELECT
    id,
    root_id,
    mime_type,
    ctime
FROM file
WHERE path = ?
`,
	query.FileGetByID: `
SELECT
    root_id,
    path,
    mime_type,
    ctime
FROM file
WHERE id = ?
`,
	query.FileGetByPattern: `
SELECT
    id,
    root_id,
    path,
    mime_type,
    ctime
FROM file
WHERE path LIKE ?
`,
	query.FileGetByRoot: `
SELECT
    id,
    path,
    mime_type,
    ctime
FROM file
WHERE root_id = ?
`,
	query.FileGetNoMeta: `
SELECT
    f.id,
    f.root_id,
    f.path,
    f.mime_type,
    f.ctime
FROM file f
LEFT OUTER JOIN meta m ON f.id = m.file_id
WHERE m.file_id IS NULL
ORDER BY f.path
`,
	query.FileGetAll: `
SELECT
    id,
    root_id,
    path,
    mime_type,
    ctime
FROM file
`,
	query.BlacklistAdd: `
INSERT INTO blacklist (pattern, is_glob)
VALUES                (      ?,       ?)
RETURNING id
`,
	query.BlacklistHit: `
UPDATE blacklist
SET hit_cnt = hit_cnt + 1
WHERE id = ?
`,
	query.BlacklistGetAll: `
SELECT
    id,
    pattern,
    is_glob,
    hit_cnt
FROM blacklist
ORDER BY hit_cnt DESC
`,
	query.MetaAdd: `
INSERT INTO meta (file_id, timestamp, content, meta)
VALUES           (      ?,         ?,       ?,    ?)
RETURNING id
`,
	query.MetaGetByFile: `
SELECT
    id,
    timestamp,
    content,
    meta
FROM meta
WHERE file_id = ?
`,
	query.MetaGetByRoot: `
SELECT
    m.id,
    m.file_id,
    m.timestamp,
    m.content,
    m.meta,
    f.path,
    f.mime_type,
    f.ctime
FROM meta m
INNER JOIN file f ON m.file_id = f.id
INNER JOIN root r ON f.root_id = r.id
WHERE r.id = ?
ORDER BY f.path
`,
	query.MetaGetOutdated: `
SELECT
    m.id,
    m.file_id,
    m.timestamp,
    m.content,
    m.meta,
    f.root_id,
    f.path,
    f.mime_type,
    f.ctime
FROM meta m
INNER JOIN file f ON m.file_id = f.id
WHERE f.ctime > m.timestamp
ORDER BY f.path
`,
	query.MetaGetAll: `
SELECT
    m.id,
    m.file_id,
    m.timestamp,
    m.content,
    m.meta,
    f.root_id,
    f.path,
    f.mime_type,
    f.ctime
FROM meta m
INNER JOIN file f ON m.file_id = f.id
ORDER BY f.path
`,
	query.MetaUpsert: `
INSERT INTO meta (file_id, timestamp, content, meta)
VALUES           (      ?,         ?,       ?,    ?)
ON CONFLICT (file_id) DO UPDATE SET
    timestamp = excluded.timestamp,
    content = excluded.content,
    meta = meta.content
WHERE file_id = excluded.file_id
RETURNING id
`,
}
