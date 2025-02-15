// /home/krylon/go/src/github.com/blicero/snoopy/walker/01_walker_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 28. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-03 18:44:18 krylon>

package walker

import (
	"testing"

	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/model"
)

func TestCreateWalker(t *testing.T) {
	var err error

	if w, err = NewWithBlacklist(blacklist.NewBlacklist()); err != nil {
		t.Fatalf("Failed to create Walker: %s",
			err.Error())
	}
} // func TestCreateWalker(t *testing.T)

func TestWalkDirectory(t *testing.T) {
	var (
		err error
		db  *database.Database
		r   = &model.Root{Path: testRoot}
	)

	db = database.Get()
	defer database.Put(db)

	if err = db.RootAdd(r); err != nil {
		t.Fatalf("Failed to add Root Directory %s to database: %s",
			r.Path,
			err.Error())
	} else if err = w.Walk(r); err != nil {
		t.Fatalf("Failed to traverse root directory %s: %s",
			testRoot,
			err.Error())
	}

	var files []*model.File

	if files, err = db.FileGetAll(); err != nil {
		t.Fatalf("Failed to load all Files from Database: %s",
			err.Error())
	} else if len(files) != fileCnt {
		t.Errorf("Unexpected number of files in Database: %d (expected %d)",
			len(files),
			fileCnt)
	}
} // func TestWalkDirectory(t *testing.T)
