// /home/krylon/go/src/github.com/blicero/snoopy/extractor/extractor.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-10 19:27:14 krylon>

// Package extractor deals with extracting (hence the name - duh!) searchable
// metadata from the files the Walker has found.
package extractor

import (
	"errors"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	"github.com/dhowden/tag"
	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/exif2"
)

const (
	bigFile = 64 * 1024 * 1024 // 64 MiB
)

// ErrTooLarge indicates that a file is too large to be processed.
var ErrTooLarge = errors.New("File is too large")

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
		raw  []byte
		meta *FileMeta
		info os.FileInfo
	)

	if info, err = os.Stat(f.Path); err != nil {
		return nil, err
	} else if info.Size() > bigFile {
		return nil, ErrTooLarge
	} else if raw, err = os.ReadFile(f.Path); err != nil {
		return nil, err
	}

	meta = &FileMeta{
		FileID:    f.ID,
		Timestamp: time.Now(),
		Content:   string(raw),
		Meta:      make(map[string]string),
		f:         f,
	}

	return meta, nil
} // func processPlaintext(f *model.File) (*FileMeta, error)

func processAudio(f *model.File) (*FileMeta, error) {
	var (
		err  error
		fh   *os.File
		meta *FileMeta
		am   tag.Metadata
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if am, err = tag.ReadFrom(fh); err != nil {
		return nil, err
	}

	meta = &FileMeta{
		FileID:    f.ID,
		Timestamp: time.Now(),
		f:         f,
	}

	meta.Meta = map[string]string{
		"Title":  am.Title(),
		"Artist": am.Artist(),
		"Album":  am.Album(),
		"Year":   strconv.FormatInt(int64(am.Year()), 10),
	}

	return meta, nil
} // func processAudio(f *model.File) (*FileMeta, error)

func processImage(f *model.File) (*FileMeta, error) {
	var (
		err  error
		fh   *os.File
		im   exif2.Exif
		meta = &FileMeta{
			FileID:    f.ID,
			Timestamp: time.Now(),
			f:         f,
		}
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if im, err = imagemeta.Decode(fh); err != nil {
		return nil, err
	}

	meta.Meta = map[string]string{
		"Date":        im.GPS.Date().Format(common.TimestampFormat),
		"Latitude":    strconv.FormatFloat(im.GPS.Latitude(), 'f', -1, 32),
		"Longitude":   strconv.FormatFloat(im.GPS.Longitude(), 'f', -1, 32),
		"XResolution": strconv.FormatUint(uint64(im.XResolution), 10),
		"YResolution": strconv.FormatUint(uint64(im.YResolution), 10),
	}

	return meta, nil
} // func processImage(f *model.File) (*FileMeta, error)
