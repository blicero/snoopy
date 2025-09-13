#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Time-stamp: <2025-09-13 16:33:08 krylon>
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
from datetime import datetime
from typing import Final, Optional

from snoopy import common
from snoopy.database import Database
from snoopy.model import Folder

test_dir: Final[str] = os.path.join(
    "/tmp",
    datetime.now().strftime("snoopy_test_database_%Y%m%d_%H%M%S"))


class TestDatabase(unittest.TestCase):
    """Test the database."""

    conn: Optional[Database] = None
    folders: list[Folder] = []

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

    def test_01_db_open(self) -> None:
        """Attempt to open a fresh database."""
        db: Database = Database()
        self.assertIsNotNone(db)
        self.db(db)

    def test_02_folder_add(self) -> None:
        """Try adding the odd folder or two."""
        db: Database = self.db()

        pathnames: Final[list[str]] = [
            "/data/Documents",
            "/data/Video",
            os.environ["HOME"],
        ]

        folders = [Folder(path=x) for x in pathnames]

        with db:
            for f in folders:
                db.folder_add(f)
                self.assertGreater(f.fid, 0)

        self.folders = folders

    def test_03_folder_update_scan(self) -> None:
        """Try updating the test folders' timestamps."""
        db: Database = self.db()
        stamp: Final[datetime] = datetime.now()

        with db:
            for f in self.folders:
                self.assertEqual(f.last_scan.timestamp(), 0.0)
                db.folder_update_scan(f, stamp)
                self.assertGreater(f.last_scan.timestamp(), 0.0)

# Local Variables: #
# python-indent: 4 #
# End: #
