// /home/krylon/go/src/github.com/blicero/snoopy/walker/walker.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-06 18:35:01 krylon>

// Package walker implements the traversal of directories and the processing
// of the files therein.
package walker

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/common/path"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	"github.com/gabriel-vasile/mimetype"
)

const (
	minSize      = 4096            // nolint: unused,deadcode // 4 KiB, one page of virtual memory
	scanInterval = time.Minute * 5 // XXX Set to more reasonable value after testing/debugging
)

// Walker traverses directory trees and processes the files it finds.
type Walker struct {
	log    *log.Logger
	bl     *blacklist.Blacklist
	db     *database.Database
	active atomic.Bool
	visitQ chan *model.Root
	ticker *time.Ticker
}

// NewWalker creates a new Walker instance that uses the given Blacklist.
func NewWalker(bl *blacklist.Blacklist) (*Walker, error) {
	var (
		err    error
		dbpath string
		w      = &Walker{bl: bl}
	)

	dbpath = common.Path(path.Database)

	if w.log, err = common.GetLogger(logdomain.Walker); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Log for Walker: %s\n",
			err.Error(),
		)
		return nil, err
	} else if w.db, err = database.Open(dbpath); err != nil {
		w.log.Printf("[ERROR] Failed to open database at %s: %s\n",
			dbpath,
			err.Error())
	}

	w.visitQ = make(chan *model.Root, 16)
	w.ticker = time.NewTicker(time.Minute)
	w.active.Store(true)
	go w.loop()

	return w, nil
} // func NewWalker(bl *blacklist.Blacklist) (*Walker, error)

// IsActive returns the Walker's Active flag
func (w *Walker) IsActive() bool {
	return w.active.Load()
} // func (w *Walker) IsActive() bool

// Stop clears the Walker's Active flag
func (w *Walker) Stop() {
	w.active.Store(false)
} // func (w *Walker) Stop()

// ScheduleScan adds the given Root to the Scan queue.
func (w *Walker) ScheduleScan(r *model.Root) {
	w.visitQ <- r
} // func (w *Walker) ScheduleScan(r *model.Root)

func (w *Walker) loop() {
	defer w.log.Println("[INFO] Quitting Walker loop")
	for w.IsActive() {
		select {
		case <-w.ticker.C:
			continue
		case r := <-w.visitQ:
			if err := w.Walk(r); err != nil {
				w.log.Printf("[ERROR] An error occurred while walking %s: %s\n",
					r.Path,
					err.Error())
			}
		}
	}
} // func (w *Walker) loop()

// Walk initiates a traversal of the given Directory tree
func (w *Walker) Walk(r *model.Root) error {
	w.log.Printf("[INFO] Walker about to traverse %s\n",
		r.Path)
	defer w.log.Printf("[INFO] Walker finished traversing %s\n", r.Path)

	if r.LastScan.Add(scanInterval).After(time.Now()) {
		w.log.Printf("[INFO] Directory %s is not due for another scan until %s\n",
			r.Path,
			r.LastScan.Add(scanInterval).Format(common.TimestampFormatMinute))
		return nil
	}

	return filepath.WalkDir(r.Path, w.generateVisitorFunc(r))
} // func (w *Walker) Walk(r *model.Root) error

func (w *Walker) generateVisitorFunc(r *model.Root) fs.WalkDirFunc {
	return func(path string, entry fs.DirEntry, incoming error) error {
		var (
			err   error
			f     *model.File
			info  fs.FileInfo
			mtype *mimetype.MIME
		)

		if incoming != nil {
			w.log.Printf("[ERROR] Incoming error %T: %s\n",
				incoming,
				incoming.Error())
		} else if w.bl.Match(path) {
			w.log.Printf("[DEBUG] %s is blacklisted, we are skipping it\n", path)
			return nil
		} else if entry.IsDir() {
			return nil
		} else if info, err = entry.Info(); err != nil {
			w.log.Printf("[ERROR] Cannot query FileInfo on %s: %s\n",
				path,
				err.Error())
			return err
		} else if f, err = w.db.FileGetByPath(path); err != nil {
			w.log.Printf("[ERROR] Failed to look up file %s by path: %s\n",
				path,
				err.Error())
			return err
		} else if f == nil {
			w.log.Printf("[TRACE] Add file %s to database...\n",
				path)

			if mtype, err = mimetype.DetectFile(path); err != nil {
				w.log.Printf("[ERROR] Failed to determine MIME type for %s: %s\n",
					path,
					err.Error())
				return err
			}

			f = &model.File{
				RootID: r.ID,
				Path:   path,
				CTime:  info.ModTime(),
				Type:   mtype.String(),
			}

			if err = w.db.FileAdd(f); err != nil {
				w.log.Printf("[ERROR] Failed to add File %s to database: %s\n",
					path,
					err.Error())
				return err
			}
		} else if info.ModTime().After(f.CTime) {
			w.log.Printf("[DEBUG] Update CTime on File %s:\nOld: %s\nNew: %s\n",
				f.Path,
				f.CTime.Format(common.TimestampFormat),
				info.ModTime().Format(common.TimestampFormat))
			w.db.FileUpdateCtime(f, info.ModTime()) // nolint: errcheck
		}

		return nil
	} // func (w *Walker) visit(path string, info fs.FileInfo, incoming error) error
} // func (w *Walker) generateVisitorFunc(r *model.Root) fs.WalkDirFunc
