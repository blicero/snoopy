// /home/krylon/go/src/github.com/blicero/snoopy/logdomain/logdomain.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-31 18:03:44 krylon>

package logdomain

//go:generate stringer -type=ID

type ID uint8

const (
	Database ID = iota
	DBPool
	Walker
	GUI
)

func AllDomains() []ID {
	return []ID{
		Database,
		DBPool,
		Walker,
		GUI,
	}
} // func AllDomains() []ID
