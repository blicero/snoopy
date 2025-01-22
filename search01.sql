-- /home/krylon/go/src/github.com/blicero/snoopy/search01.sql
-- Time-stamp: <2025-01-22 15:28:41 krylon>
-- created on 22. 01. 2025 by Benjamin Walkenhorst
-- (c) 2025 Benjamin Walkenhorst
-- Use at your own risk!

SELECT
    f.path,
    f.mime_type,
    DATETIME(f.ctime, 'unixepoch')  AS ctime
FROM logothek l
INNER JOIN file f ON l.file_id = f.id
WHERE l.body MATCH 'emacs OR openbsd'
;

