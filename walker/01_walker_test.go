// /home/krylon/go/src/github.com/blicero/snoopy/walker/01_walker_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 28. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-28 17:49:51 krylon>

package walker

import (
	"testing"

	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/model"
)

func TestCreateWalker(t *testing.T) {
	var err error

	if w, err = NewWalker(blacklist.NewBlacklist()); err != nil {
		t.Fatalf("Failed to create Walker: %s",
			err.Error())
	}
} // func TestCreateWalker(t *testing.T)

func TestWalkDirectory(t *testing.T) {
	var (
		err error
		r   = &model.Root{Path: testRoot}
	)

	if err = w.db.RootAdd(r); err != nil {
		t.Fatalf("Failed to add Root Directory %s to database: %s",
			r.Path,
			err.Error())
	} else if err = w.Walk(r); err != nil {
		t.Fatalf("Failed to traverse root directory %s: %s",
			testRoot,
			err.Error())
	}
} // func TestWalkDirectory(t *testing.T)
