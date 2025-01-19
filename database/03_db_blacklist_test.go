// /home/krylon/go/src/github.com/blicero/snoopy/database/03_db_blacklist_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-19 19:15:26 krylon>

package database

import (
	"testing"

	"github.com/blicero/snoopy/blacklist"
)

var blTestItems []blacklist.Item

func TestBlacklistItemAdd(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	type testCase struct {
		pattern     string
		isGlob      bool
		expectError bool
	}

	var testCases = []testCase{
		{
			pattern: "#$",
		},
		{
			pattern:     "[.](tmp|bak|log|err|out",
			expectError: true,
		},
	}

	blTestItems = make([]blacklist.Item, 0, len(testCases))

	for _, c := range testCases {
		var (
			err  error
			item blacklist.Item
		)

		if c.isGlob {
			if item, err = blacklist.NewGlobItem(0, 0, c.pattern); err != nil {
				if !c.expectError {
					t.Errorf("Failed to compile blacklist pattern %q: %s",
						c.pattern,
						err.Error())
				}
				continue
			}
		} else if item, err = blacklist.NewReItem(0, 0, c.pattern); err != nil {
			if !c.expectError {
				t.Errorf("Failed to compile regex blacklist pattern %q: %s",
					c.pattern,
					err.Error())
			}
			continue
		}

		if err = db.BlacklistAdd(item); err != nil {
			if !c.expectError {
				t.Errorf("Failed to add blacklist pattern %q to database: %s",
					c.pattern,
					err.Error())
			}
			continue
		}

		blTestItems = append(blTestItems, item)
	}
} // func TestBlacklistItemAdd(t *testing.T)

func TestBlacklistGetAll(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var (
		err   error
		items []blacklist.Item
	)

	if items, err = db.BlacklistGetAll(); err != nil {
		t.Fatalf("Failed to load all Blacklist Items: %s",
			err.Error())
	} else if len(items) != len(blTestItems)+10 {
		t.Fatalf("Unexpected number of Items returned by db.BlacklistGetAll: %d (expected %d)",
			len(items),
			len(blTestItems))
	}

	blTestItems = items
} // func TestBlacklistGetAll(t *testing.T)

func TestBlacklistHit(t *testing.T) {
	if db == nil {
		t.SkipNow()
	}

	var err error

	for _, item := range blTestItems {
		if err = db.BlacklistHit(item); err != nil {
			t.Errorf("Failed to increase hit count for Item %d (%s): %s",
				item.GetID(),
				item.GetPattern(),
				err.Error())
		}
	}

	var items []blacklist.Item

	if items, err = db.BlacklistGetAll(); err != nil {
		t.Fatalf("Failed to load all Blacklist Items: %s",
			err.Error())
	}

	for _, i := range items {
		if i.HitCount() != 1 {
			t.Errorf("Unexpected HitCount for Item %d (%s): %d (expected 1)",
				i.GetID(),
				i.GetPattern(),
				i.HitCount())
		}
	}
} // func TestBlacklistHit(t *testing.T)
