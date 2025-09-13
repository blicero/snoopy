#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Time-stamp: <2025-09-13 16:31:12 krylon>
#
# /data/code/python/snoopy/model.py
# created on 12. 09. 2025
# (c) 2025 Benjamin Walkenhorst
#
# This file is part of the PyKuang network scanner. It is distributed under the
# terms of the GNU General Public License 3. See the file LICENSE for details
# or find a copy online at https://www.gnu.org/licenses/gpl-3.0

"""
snoopy.model

(c) 2025 Benjamin Walkenhorst
"""

import re
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum, auto
from typing import Final, Optional


class FileType(Enum):
    """FileType defines constants for identifying various types of files."""

    Text = auto()
    PDF = auto()
    Image = auto()
    Document = auto()
    Other = auto()


@dataclass(slots=True, kw_only=True)
class Folder:
    """Folder is a directory we care about."""

    fid: int = 0
    path: str
    last_scan: datetime = field(default=datetime.fromtimestamp(0))


suffix_pat: Final[re.Pattern] = re.compile(r"[.](\w+)$")


@dataclass(slots=True, kw_only=True)
class File:
    """File is a file we may search for."""

    fid: int = 0
    folder_id: int
    path: str
    mime_type: str = "application/octet-stream"
    stime: Optional[datetime] = None
    size: int
    content: str

    @property
    def suffix(self) -> str:
        """Return the file name suffix."""
        m = suffix_pat.search(self.path)
        if m is None:
            return ""
        return m[1]

# Local Variables: #
# python-indent: 4 #
# End: #
