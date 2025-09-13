#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Time-stamp: <2025-09-13 16:31:23 krylon>
#
# /data/code/python/snoopy/database.py
# created on 13. 09. 2025
# (c) 2025 Benjamin Walkenhorst
#
# This file is part of the PyKuang network scanner. It is distributed under the
# terms of the GNU General Public License 3. See the file LICENSE for details
# or find a copy online at https://www.gnu.org/licenses/gpl-3.0

"""
snoopy.database

(c) 2025 Benjamin Walkenhorst
"""

import logging
import sqlite3
from datetime import datetime
from enum import Enum, auto
from threading import Lock
from typing import Final, Optional

import krylib

from snoopy import common
from snoopy.model import File, Folder

qinit: Final[list[str]] = [
    """
CREATE TABLE folder (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    last_scan INTEGER NOT NULL DEFAULT 0
) STRICT
    """,
    "CREATE INDEX folder_path_idx ON folder (path)",
    """
CREATE TABLE file (
    id INTEGER PRIMARY KEY,
    folder_id INTEGER NOT NULL,
    path TEXT UNIQUE NOT NULL,
    mime_type TEXT NOT NULL,
    stime INTEGER NOT NULL DEFAULT 0,
    size INTEGER NOT NULL,
    content TEXT,
    CHECK (size >= 0),
    FOREIGN KEY (folder_id) REFERENCES folder (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE
) STRICT
    """,
    "CREATE INDEX file_folder_idx ON file (folder_id)",
    "CREATE INDEX file_path_idx ON file (path)",
    "CREATE INDEX file_stime_idx ON file (stime)",
    "CREATE INDEX file_mime_idx ON file (mime_type)",
]


class Query(Enum):
    """Query identifies a particulary query on the database."""

    FolderAdd = auto()
    FolderUpdateScan = auto()
    FolderGetAll = auto()
    FileAdd = auto()
    FileUpdate = auto()
    FileDelete = auto()
    FileGetByRoot = auto()
    FileGetByPath = auto()


qdb: Final[dict[Query, str]] = {
    Query.FolderAdd: "INSERT INTO folder (path) VALUES (?) RETURNING id",
    Query.FolderUpdateScan: "UPDATE folder SET last_scan = ? WHERE id = ?",
    Query.FolderGetAll: """
SELECT
    id,
    path,
    last_scan
FROM folder
    """,
    Query.FileAdd: """
INSERT INTO file (folder_id, path, mime_type, size)
          VALUES (        ?,    ?          ?,    ?)
RETURNING id
    """,
    Query.FileUpdate: """
UPDATE file SET stime = ?, size = ?, content = ? WHERE id = ?
    """,
    Query.FileDelete: "DELETE FROM file WHERE id = ?",
    Query.FileGetByRoot: """
SELECT
    id,
    path,
    mime_type,
    stime,
    size,
    content
FROM file WHERE folder_id = ?
ORDER BY path
    """,
    Query.FileGetByPath: """
SELECT
    id,
    folder_id
    mime_type,
    stime,
    size,
    content
FROM file WHERE path = ?
    """,
}


open_lock: Final[Lock] = Lock()


class Database:
    """Database wraps the database connection and the operations we perform on it."""

    __slots__ = [
        "db",
        "log",
        "path",
    ]

    log: logging.Logger
    db: sqlite3.Connection
    path: str

    def __init__(self, path: Optional[str] = None) -> None:
        if path is None:
            self.path = common.path.db
        else:
            self.path = path

        self.log = common.get_logger("database")
        self.log.debug("Open database at %s", self.path)

        with open_lock:
            exist: Final[bool] = krylib.fexist(self.path)
            self.db = sqlite3.connect(self.path)
            self.db.isolation_level = None

            cur: Final[sqlite3.Cursor] = self.db.cursor()
            cur.execute("PRAGMA foreign_keys = true")
            cur.execute("PRAGMA journal_mode = WAL")

            if not exist:
                self.__create_db()

    def __create_db(self) -> None:
        """Initialize a freshly created database"""
        self.log.debug("Initialize fresh database at %s", self.path)
        with self.db:
            for query in qinit:
                cur: sqlite3.Cursor = self.db.cursor()
                cur.execute(query)
        self.log.debug("Database initialized successfully.")

    def __enter__(self) -> None:
        self.db.__enter__()

    def __exit__(self, ex_type, ex_val, tb):
        return self.db.__exit__(ex_type, ex_val, tb)

    def folder_add(self, f: Folder) -> None:
        """Add a new Folder to the Database."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FolderAdd], (f.path, ))
        row = cur.fetchone()
        f.fid = row[0]

    def folder_update_scan(self, f: Folder, stamp: datetime) -> None:
        """Update a Folder's last_scan timestamp."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FolderUpdateScan], (int(stamp.timestamp()), f.fid))
        f.last_scan = stamp

    def folder_get_all(self) -> list[Folder]:
        """Load all Folders from the database."""
        cur = self.db.cursor()
        folders: list[Folder] = []

        cur.execute(qdb[Query.FolderGetAll])

        for row in cur:
            f: Folder = Folder(fid=row[0], path=row[1], last_scan=datetime.fromtimestamp(row[2]))
            folders.append(f)

        return folders

    def file_add(self, f: File) -> None:
        """Add a File to the database."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FileAdd], (f.folder_id,
                                         f.path,
                                         f.mime_type,
                                         f.size))
        row = cur.fetchone()
        f.fid = row[0]

    def file_update(self,
                    f: File,
                    stime: datetime,
                    size: int,
                    content: str) -> None:
        """Update a File."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FileUpdate],
                    (int(stime.timestamp()),
                     size,
                     content,
                     f.fid))
        f.stime = stime
        f.size = size
        f.content = content

    def file_delete(self, f: File) -> None:
        """Delete a File from the database."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FileDelete], (f.fid, ))

    def file_get_by_root(self, fldr: Folder) -> list[File]:
        """Get all Files from a particular Folder."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FileGetByRoot], (fldr.fid, ))

        files: list[File] = []

        for row in cur:
            f = File(fid=row[0],
                     folder_id=fldr.fid,
                     path=row[1],
                     mime_type=row[2],
                     stime=datetime.fromtimestamp(row[3]),
                     size=row[5],
                     content=row[6])
            files.append(f)

        return files

    def file_get_by_path(self, path: str) -> Optional[File]:
        """Get a File by its path."""
        cur = self.db.cursor()
        cur.execute(qdb[Query.FileGetByPath], (path, ))

        row = cur.fetchone()
        if row is None:
            self.log.debug("File \"%s\" was not found in database.", path)
            return None

        f: File = File(fid=row[0],
                       folder_id=row[1],
                       path=path,
                       mime_type=row[2],
                       stime=datetime.fromtimestamp(row[3]),
                       size=row[5],
                       content=row[6])
        return f

# Local Variables: #
# python-indent: 4 #
# End: #
