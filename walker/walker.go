// /home/krylon/go/src/github.com/blicero/snoopy/walker/walker.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-03 18:43:35 krylon>

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
	active atomic.Bool
	visitQ chan *model.Root
	ticker *time.Ticker
}

// NewWithBlacklist creates a new Walker instance that uses the given Blacklist.
func NewWithBlacklist(bl *blacklist.Blacklist) (*Walker, error) {
	var (
		err error
		w   = &Walker{bl: bl}
	)

	if w.log, err = common.GetLogger(logdomain.Walker); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Log for Walker: %s\n",
			err.Error(),
		)
		return nil, err
	}

	w.visitQ = make(chan *model.Root, 16)
	w.ticker = time.NewTicker(time.Minute)
	w.active.Store(true)
	go w.loop()

	return w, nil
} // func NewWithBlacklist(bl *blacklist.Blacklist) (*Walker, error)

// New creates a new Walker instance that uses the blacklist from the database.
func New() (*Walker, error) {
	var (
		err error
		w   = new(Walker)
	)

	if w.log, err = common.GetLogger(logdomain.Walker); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Log for Walker: %s\n",
			err.Error(),
		)
		return nil, err
	}

	var (
		db    = database.Get()
		items []blacklist.Item
	)

	defer database.Put(db)

	if items, err = db.BlacklistGetAll(); err != nil {
		w.log.Printf("[ERROR] Failed to load all blacklist Items: %s\n",
			err.Error())
		return nil, err
	}

	w.bl = blacklist.NewBlacklist(items...)

	w.visitQ = make(chan *model.Root, 16)
	w.ticker = time.NewTicker(time.Minute)
	w.active.Store(true)
	go w.loop()

	return w, nil
} // func NewWithBlacklist(bl *blacklist.Blacklist) (*Walker, error)

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

	var (
		db  *database.Database
		err error
	)

	db = database.Get()
	defer database.Put(db)

	if err = filepath.WalkDir(r.Path, w.generateVisitorFunc(r)); err != nil {
		w.log.Printf("[ERROR] Encountered an error scanning Root %s: %s\n",
			r.Path,
			err.Error())
	} else if err = db.RootMarkScan(r, time.Now()); err != nil {
		w.log.Printf("[ERROR] Failed to update scan timestamp on Root %s (%d): %s\n",
			r.Path,
			r.ID,
			err.Error())
	}

	return err
} // func (w *Walker) Walk(r *model.Root) error

func (w *Walker) generateVisitorFunc(r *model.Root) fs.WalkDirFunc {
	return func(path string, entry fs.DirEntry, incoming error) error {
		var (
			err   error
			f     *model.File
			ftype fs.FileMode
			info  fs.FileInfo
			mtype *mimetype.MIME
			db    *database.Database
		)

		db = database.Get()
		defer database.Put(db)

		if incoming != nil {
			w.log.Printf("[ERROR] Incoming error %T: %s\n",
				incoming,
				incoming.Error())
		} else if w.bl.Match(path) {
			// FIXME We need to increment the hit counter in the
			//       database for the item that matched, which means
			//       we also need to know which Item matched.
			w.log.Printf("[DEBUG] %s is blacklisted, we are skipping it\n", path)
			return nil
		} else if entry.IsDir() {
			return nil
		} else if ftype = entry.Type(); ftype&fs.ModeType != 0 {
			// Not a regular file: Skip it!
			return nil
		} else if info, err = entry.Info(); err != nil {
			w.log.Printf("[ERROR] Cannot query FileInfo on %s: %s\n",
				path,
				err.Error())
			return err
		} else if f, err = db.FileGetByPath(path); err != nil {
			w.log.Printf("[ERROR] Failed to look up file %s by path: %s\n",
				path,
				err.Error())
			return err
		} else if f == nil {
			if mtype, err = mimetype.DetectFile(path); err != nil {
				if err.Error() == "permission denied" {
					return nil
				}

				w.log.Printf("[ERROR] Failed to determine MIME type for %s: %s\n",
					path,
					err.Error())
				return err
			}

			w.log.Printf("[TRACE] Add file %s to database...\n",
				path)

			f = &model.File{
				RootID: r.ID,
				Path:   path,
				CTime:  info.ModTime(),
				Type:   mtype.String(),
			}

			if err = db.FileAdd(f); err != nil {
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
			db.FileUpdateCtime(f, info.ModTime()) // nolint: errcheck
		}

		return nil
	} // func (w *Walker) visit(path string, info fs.FileInfo, incoming error) error
} // func (w *Walker) generateVisitorFunc(r *model.Root) fs.WalkDirFunc
