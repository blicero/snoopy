// /home/krylon/go/src/github.com/blicero/snoopy/model/model.go
// -*- mode: go; coding: utf-8; -*-
// Created on 22. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-10 22:17:58 krylon>

// Package model provides the data types used throughout the application.
package model

import (
	"time"
)

// Root is a directory that is scanned for files
type Root struct {
	ID       int64     `json:"id"`
	Path     string    `json:"path"`
	LastScan time.Time `json:"last_scan"`
}

// File is a ... file.
type File struct {
	ID          int64     `json:"id"`
	RootID      int64     `json:"root_id"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	LastRefresh time.Time `json:"last_refresh"`
	Size        int64     `json:"size"`
	CTime       time.Time `json:"ctime"`
}

// FileMeta represents the metadata extracted from a File.
//
// F, if non-nil, shall be a pointer to the File the metadata refers to.
// This is mainly intended for convenience, there probably should be a
// a more elegant solution.
type FileMeta struct {
	ID        int64             `json:"id"`
	FileID    int64             `json:"file_id"`
	Timestamp time.Time         `json:"timestamp"`
	Content   string            `json:"content"`
	Meta      map[string]string `json:"meta"`
	F         *File             `json:"-"`
}
