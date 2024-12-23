// /home/krylon/go/src/github.com/blicero/snoopy/database/qdb.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-23 20:47:12 krylon>

package database

import "github.com/blicero/snoopy/database/query"

var dbQueries = map[query.ID]string{}
