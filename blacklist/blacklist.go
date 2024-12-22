// /home/krylon/go/src/github.com/blicero/snoopy/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 22. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-22 17:55:09 krylon>

// Package blacklist provides a filter meant to exclude files from scanning.
package blacklist

import (
	"regexp"

	"github.com/gobwas/glob"
)

// Item is the common interface for several types of Blacklist patterns.
type Item interface {
	Match(path string) bool
	HitCount() int64
}

// ReItem matches filenames against a regular expression.
type ReItem struct {
	Pattern *regexp.Regexp
	Count   int64
}

func (i *ReItem) Match(path string) bool {
	var hit = i.Pattern.MatchString(path)
	if hit {
		i.Count++
	}
	return hit
} // func (i *ReItem) Match(path string) bool

func (i *ReItem) HitCount() int64 {
	return i.Count
} // func (i *ReItem) HitCount() int64

// GlobItem matches paths using the well-known globbing mechanism.
type GlobItem struct {
	Pattern glob.Glob
	Count   int64
}

func (i *GlobItem) Match(path string) bool {
	var hit = i.Pattern.Match(path)
	if hit {
		i.Count++
	}
	return hit
} // func (i *GlobItem) Match(path string) bool

func (i *GlobItem) HitCount() int64 {
	return i.Count
} // func (i *GlobItem) HitCount() int64

type Blacklist struct {
	Patterns []Item
}
