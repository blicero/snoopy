// /home/krylon/go/src/github.com/blicero/snoopy/extractor/extractor.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-08 18:05:50 krylon>

// Package extractor deals with extracting (hence the name - duh!) searchable
// metadata from the files the Walker has found.
package extractor

import (
	"log"
	"runtime"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/logdomain"
)

// Extractor wraps the handling of various file types to extract searchable
// metadata / content from.
type Extractor struct {
	log  *log.Logger
	pool *database.Pool
}

// New creates a new Extractor.
func New() (*Extractor, error) {
	var (
		err error
		ex  = new(Extractor)
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

// func (ex *Extractor) Process(f *model.File) error {

// } // func (ex *Extractor) Process(f *model.File) error
