// /home/krylon/go/src/github.com/blicero/snoopy/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 22. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-04 14:52:01 krylon>

// Package blacklist provides a filter meant to exclude files from scanning.
package blacklist

import (
	"regexp"
	"sort"
	"sync"

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

// NewReItem creates a new Regexp-based Item
func NewReItem(pattern string) (Item, error) {
	var (
		err  error
		item = new(ReItem)
	)

	if item.Pattern, err = regexp.Compile(pattern); err != nil {
		return nil, err
	}

	return item, nil
} // func NewReItem(pattern string) Item

// Match matches the given path against the Item.
func (i *ReItem) Match(path string) bool {
	var hit = i.Pattern.MatchString(path)
	if hit {
		i.Count++
	}
	return hit
} // func (i *ReItem) Match(path string) bool

// HitCount returns the number of times the Item has matched a path successfully
func (i *ReItem) HitCount() int64 {
	return i.Count
} // func (i *ReItem) HitCount() int64

// GlobItem matches paths using the well-known globbing mechanism.
type GlobItem struct {
	Pattern glob.Glob
	Count   int64
}

// NewGlobItem creates a new GlobItem from the given pattern
func NewGlobItem(pattern string) (Item, error) {
	var (
		err  error
		item = new(GlobItem)
	)

	if item.Pattern, err = glob.Compile(pattern); err != nil {
		return nil, err
	}

	return item, nil
} // func NewGlobItem(pattern string) Item

// Match matches the given path against the Item.
func (i *GlobItem) Match(path string) bool {
	var hit = i.Pattern.Match(path)
	if hit {
		i.Count++
	}
	return hit
} // func (i *GlobItem) Match(path string) bool

// HitCount returns the number of times the Item has matched a path successfully
func (i *GlobItem) HitCount() int64 {
	return i.Count
} // func (i *GlobItem) HitCount() int64

// Blacklist wraps a number of Items.
type Blacklist struct {
	lock     sync.RWMutex
	Patterns []Item
}

// NewBlacklist creates a new Blacklist from the given patterns.
func NewBlacklist(patterns ...Item) *Blacklist {
	var b = &Blacklist{
		Patterns: make([]Item, len(patterns)),
	}

	copy(b.Patterns, patterns)

	return b
} // func NewBlacklist(patterns... Item) *Blacklist

func (b *Blacklist) Len() int           { return len(b.Patterns) }
func (b *Blacklist) Swap(i, j int)      { b.Patterns[i], b.Patterns[j] = b.Patterns[j], b.Patterns[i] }
func (b *Blacklist) Less(i, j int) bool { return b.Patterns[i].HitCount() > b.Patterns[j].HitCount() }

// Match tries to match the given path against all patterns in the Blacklist,
// until either one matches or the list is exhausted.
func (b *Blacklist) Match(path string) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	for _, i := range b.Patterns {
		if i.Match(path) {
			b.lock.RUnlock()
			b.lock.Lock()
			sort.Sort(b)
			b.lock.Unlock()
			b.lock.RLock()
			return true
		}
	}

	return false
} // func (b *Blacklist) Match(path string) bool
