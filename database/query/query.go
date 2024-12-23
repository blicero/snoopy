// /home/krylon/go/src/github.com/blicero/snoopy/database/query/query.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-23 23:37:44 krylon>

//go:generate stringer -type=ID

// Package query provides symbolic constants to identify the various database
// queries we may perform.
package query

// ID identifies database queries
type ID uint8

const (
	RootAdd ID = iota
	RootGetByPath
	RootGetByID
	RootGetAll
	RootDelete
	RootMarkScan
	FileAdd
	FileDelete
	FileUpdateCtime
	FileGetByID
	FileGetByPath
	FileGetByPattern
	FileGetAll
	BlacklistAdd
	BlacklistHit
)
