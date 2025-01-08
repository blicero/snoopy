// /home/krylon/go/src/github.com/blicero/snoopy/extractor/extractor.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-08 22:57:53 krylon>

// Package extractor deals with extracting (hence the name - duh!) searchable
// metadata from the files the Walker has found.
package extractor

import (
	"log"
	"runtime"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
)

const (
	bigFile = 64 * 1024 * 1024 // 64 MiB
)

// FileMeta represents the metadata extracted from a File.
type FileMeta struct {
	ID        int64             `json:"id"`
	FileID    int64             `json:"file_id"`
	Timestamp time.Time         `json:"timestamp"`
	Content   string            `json:"content"`
	Meta      map[string]string `json:"meta"`
	f         *model.File
}

type specialist func(*model.File) (*FileMeta, error)

// Extractor wraps the handling of various file types to extract searchable
// metadata / content from.
type Extractor struct {
	log      *log.Logger
	pool     *database.Pool
	handlers map[string]specialist
}

// New creates a new Extractor.
func New() (*Extractor, error) {
	var (
		err error
		ex  = &Extractor{
			handlers: make(map[string]specialist),
		}
	)

	if ex.log, err = common.GetLogger(logdomain.Extractor); err != nil {
		return nil, err
	} else if ex.pool, err = database.NewPool(runtime.NumCPU()); err != nil {
		ex.log.Printf("[ERROR] Failed to create Database Pool: %s\n",
			err.Error())
		return nil, err
	}

	return ex, nil
} // func New() (*Extractor, error)

// Process attempts to extract usable information from a file to use in a
// search index.
func (ex *Extractor) Process(f *model.File) (*FileMeta, error) {
	return nil, krylib.ErrNotImplemented
} // func (ex *Extractor) Process(f *model.File) (*FileMeta, error)

func processPlaintext(f *model.File) (*FileMeta, error) {
	var (
		err  error
		data []byte
		meta *FileMeta
	)

} // func processPlaintext(f *model.File) (*FileMeta, error)
