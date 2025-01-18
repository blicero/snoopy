// /home/krylon/go/src/github.com/blicero/snoopy/database/qinit.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-18 18:36:45 krylon>

package database

var initQueries = []string{
	`
CREATE TABLE root (
    id		INTEGER PRIMARY KEY,
    path	TEXT UNIQUE NOT NULL,
    last_scan	INTEGER NOT NULL DEFAULT 0
) STRICT
`,
	"CREATE INDEX root_scan_idx ON root (last_scan)",
	"CREATE UNIQUE INDEX root_path_idx ON root (path)",
	`
CREATE TABLE file (
    id		INTEGER PRIMARY KEY,
    root_id	INTEGER NOT NULL,
    path	TEXT NOT NULL,
    mime_type	TEXT NOT NULL,
    ctime	INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (root_id) REFERENCES root (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,
    UNIQUE (root_id, path)
) STRICT
`,
	"CREATE INDEX file_time_idx ON file (ctime)",
	"CREATE INDEX file_path_idx ON file (path)",
	`
CREATE TABLE blacklist (
    id		INTEGER PRIMARY KEY,
    pattern	TEXT UNIQUE NOT NULL,
    is_glob	INTEGER NOT NULL,
    hit_cnt	INTEGER NOT NULL DEFAULT 0
) STRICT
`,
	"CREATE INDEX bl_cnt_idx ON blacklist (hit_cnt)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (1,'^[.]',0,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (2,'~$',0,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (3,'*.bak',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (4,'*.out',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (5,'*.tmp',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (6,'#*',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (7,'bak.*',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (8,'*_string.go',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (9,'*_gen.go',1,0)",
	"INSERT INTO blacklist (id,pattern,is_glob,hit_cnt) VALUES (10,'*ffjson.go',1,0)",
	`
CREATE TABLE meta (
    id		INTEGER PRIMARY KEY,
    file_id	INTEGER UNIQUE NOT NULL,
    timestamp	INTEGER NOT NULL,
    content	TEXT NOT NULL DEFAULT '',
    meta	TEXT NOT NULL DEFAULT '',
    CHECK (json_valid(meta)),
    FOREIGN KEY (file_id) REFERENCES file (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE
) STRICT
`,
	"CREATE INDEX meta_time_idx ON meta (timestamp)",
	"CREATE INDEX meta_meta_idx ON meta (meta <> '')",
}
