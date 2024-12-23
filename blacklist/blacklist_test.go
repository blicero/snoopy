// /home/krylon/go/src/github.com/blicero/snoopy/blacklist/blacklist_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-23 19:56:22 krylon>

package blacklist

import (
	"path/filepath"
	"testing"
)

var bl *Blacklist

func TestCreatePatterns(t *testing.T) {
	type testCase struct {
		isGlob      bool
		pattern     string
		expectError bool
	}

	var testCases = []testCase{
		{
			isGlob:  true,
			pattern: ".*",
		},
		{
			pattern: "#$",
		},
	}

	var items = make([]Item, 0, len(testCases))

	for _, c := range testCases {
		var (
			err  error
			kind string
			item Item
		)

		if c.isGlob {
			kind = "Glob"
			item, err = NewGlobItem(c.pattern)
		} else {
			kind = "Regex"
			item, err = NewReItem(c.pattern)
		}

		if err != nil {
			if !c.expectError {
				t.Errorf("Error compiling %s pattern %q: %s",
					kind,
					c.pattern,
					err.Error())
			}
		} else if item == nil {
			t.Errorf("Failed to compile %s pattern %q",
				kind,
				c.pattern)
		} else {
			items = append(items, item)
		}
	}

	bl = NewBlacklist(items...)
} // func TestCreatePatterns(t *testing.T)

func TestMatch(t *testing.T) {
	type testCase struct {
		path        string
		shouldMatch bool
	}

	var testCases = []testCase{
		{
			path:        "/data/code/python/random_project/#random_file.py#",
			shouldMatch: true,
		},
		{
			path:        "/data/code/python/random_project/.gitignore",
			shouldMatch: true,
		},
	}

	for _, c := range testCases {
		var m = bl.Match(filepath.Base(c.path))

		if c.shouldMatch != m {
			t.Errorf("Unexpected result matching path %q against Blacklist: %t",
				c.path,
				m)
		}
	}
} // func TestMatch(t *testing.T)
