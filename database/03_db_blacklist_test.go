// /home/krylon/go/src/github.com/blicero/snoopy/database/03_db_blacklist_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-07 11:24:49 krylon>

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
			pattern: "^[.]",
		},
		{
			pattern: "#$",
		},
		{
			pattern: "bak.*",
			isGlob:  true,
		},
		{
			pattern: "*.bak",
			isGlob:  true,
		},
		{
			pattern: "~$",
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
					continue
				}
			}
		} else if item, err = blacklist.NewReItem(0, 0, c.pattern); err != nil {
			if !c.expectError {
				t.Errorf("Failed to compile regex blacklist pattern %q: %s",
					c.pattern,
					err.Error())
				continue
			}
		} else if err = db.BlacklistAdd(item); err != nil {
			if !c.expectError {
				t.Errorf("Failed to add blacklist pattern %q to database: %s",
					c.pattern,
					err.Error())
				continue
			}
		}

		blTestItems = append(blTestItems, item)
	}
} // func TestBlacklistItemAdd(t *testing.T)
