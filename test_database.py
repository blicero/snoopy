#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Time-stamp: <2025-09-16 17:02:14 krylon>
#
# /data/code/python/snoopy/test_database.py
# created on 13. 09. 2025
# (c) 2025 Benjamin Walkenhorst
#
# This file is part of the PyKuang network scanner. It is distributed under the
# terms of the GNU General Public License 3. See the file LICENSE for details
# or find a copy online at https://www.gnu.org/licenses/gpl-3.0

"""
snoopy.test_database

(c) 2025 Benjamin Walkenhorst
"""

import os
import shutil
import unittest
from abc import ABCMeta
from datetime import datetime
from typing import Final, Optional

from snoopy import common
from snoopy.database import Database
from snoopy.model import File, Folder

test_dir: Final[str] = os.path.join(
    "/tmp",
    datetime.now().strftime("snoopy_test_database_%Y%m%d_%H%M%S"))


class TestDatabase(unittest.TestCase, metaclass=ABCMeta):
    """Base class for testing the Database"""

    conn: Optional[Database] = None
    _folders: list[Folder] = []

    @classmethod
    def setUpClass(cls) -> None:
        """Prepare the testing environment."""
        common.set_basedir(test_dir)

    @classmethod
    def tearDownClass(cls) -> None:
        """Clean up afterwards."""
        shutil.rmtree(test_dir, ignore_errors=True)

    @classmethod
    def db(cls, db: Optional[Database] = None) -> Database:
        """Set or return the database."""
        if db is not None:
            cls.conn = db
            return db
        if cls.conn is not None:
            return cls.conn

        raise ValueError("No Database connection exists")

    @classmethod
    def folders(cls, folders: Optional[list[Folder]] = None) -> list[Folder]:
        """Set or return the list of Folders."""
        if folders is not None:
            cls._folders = folders

        return cls._folders

    def test_01_db_open(self) -> None:
        """Attempt to open a fresh database."""
        db: Database = Database()
        self.assertIsNotNone(db)
        self.db(db)


class TestDatabaseFolders(TestDatabase):
    """Test dealing with Folders."""

    def test_02_folder_add(self) -> None:
        """Try adding the odd folder or two."""
        db: Database = self.db()

        pathnames: Final[list[str]] = [
            "/data/Documents",
            "/data/Video",
            os.environ["HOME"],
        ]

        self.folders([Folder(path=x) for x in pathnames])

        with db:
            for f in self.folders():
                db.folder_add(f)
                self.assertGreater(f.fid, 0)

        # self.folders = folders

    def test_03_folder_update_scan(self) -> None:
        """Try updating the test folders' timestamps."""
        db: Database = self.db()
        stamp: Final[datetime] = datetime.now()

        with db:
            for f in self.folders():
                self.assertEqual(f.last_scan.timestamp(), 0.0)
                db.folder_update_scan(f, stamp)
                self.assertGreater(f.last_scan.timestamp(), 0.0)

    def test_04_folder_get_all(self) -> None:
        """Try fetching all folders from the database."""
        db: Database = self.db()

        with db:
            folders: list[Folder] = db.folder_get_all()
            self.assertIsNotNone(folders)
            self.assertEqual(len(self.folders()), len(folders))


class TestDatabaseFiles(TestDatabase):
    """Test dealing with Files."""

    _files: list[File] = []

    @classmethod
    def files(cls, files: Optional[list[File]] = None) -> list[File]:
        """Get or set the list of Files."""
        if files is not None:
            cls._files = files

        return cls._files

    def test_02_file_add(self) -> None:
        """Try adding some Files."""

# Local Variables: #
# python-indent: 4 #
# End: #
