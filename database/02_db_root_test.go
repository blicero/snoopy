// /home/krylon/go/src/github.com/blicero/snoopy/database/02_db_root_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 27. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-27 18:12:50 krylon>

package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/model"
)

const rootCnt = 16

var testRoots = make([]*model.Root, 0, rootCnt)

func TestRootAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err    error
		status bool
	)

	if err = db.Begin(); err != nil {
		t.Fatalf("Failed to start transaction: %s", err.Error())
	}

	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
		}
	}()

	for i := 0; i < rootCnt; i++ {
		var r = &model.Root{Path: fmt.Sprintf("/test/folder%03d", i+1)}

		if err = db.RootAdd(r); err != nil {
			t.Fatalf("Failed to add Root directory %s: %s",
				r.Path,
				err.Error())
		} else if r.ID == 0 {
			t.Fatalf("Newly added Root %s has no database ID", r.Path)
		}

		testRoots = append(testRoots, r)
	}

	status = true
} // func TestRootAdd(t *testing.T)

func TestRootMarkScan(t *testing.T) {
	if db == nil || len(testRoots) == 0 {
		t.SkipNow()
	}

	var (
		err    error
		status bool
		stamp  = time.Now()
	)

	if err = db.Begin(); err != nil {
		t.Fatalf("Failed to start transaction: %s", err.Error())
	}

	defer func() {
		if status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
		}
	}()

	for _, r := range testRoots {
		if err = db.RootMarkScan(r, stamp); err != nil {
			t.Fatalf("Failed to update LastScan for Root %d (%s): %s",
				r.ID,
				r.Path,
				err.Error())
		} else if r.LastScan.Unix() != stamp.Unix() {
			t.Errorf("Unexpected timestamp on Root %s: %s (expected %s)",
				r.Path,
				r.LastScan.Format(common.TimestampFormat),
				stamp.Format(common.TimestampFormat))
		}
	}

	status = true
} // func TestRootMarkScan(t *testing.T)
