// /home/krylon/go/src/github.com/blicero/snoopy/blacklist/blacklist.go
// -*- mode: go; coding: utf-8; -*-
// Created on 22. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-07 18:12:23 krylon>

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
	GetID() int64
	GetPattern() string
	IsGlob() bool
	Match(path string) bool
	HitCount() int64
}

// ReItem matches filenames against a regular expression.
type ReItem struct {
	ID      int64
	Pattern *regexp.Regexp
	Count   int64
}

// NewReItem creates a new Regexp-based Item
func NewReItem(id, cnt int64, pattern string) (Item, error) {
	var (
		err  error
		item = &ReItem{ID: id, Count: cnt}
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

// GetID returns the Item's ID
func (i *ReItem) GetID() int64 {
	return i.ID
} // func (i *ReItem) GetID() int64

// GetPattern returns the Item's pattern string
func (i *ReItem) GetPattern() string {
	return i.Pattern.String()
} // func (i *ReItem) GetPattern() string

// IsGlob returns true if the Item uses globbing (i.e. false)
func (i *ReItem) IsGlob() bool { return false }

// HitCount returns the number of times the Item has matched a path successfully
func (i *ReItem) HitCount() int64 {
	return i.Count
} // func (i *ReItem) HitCount() int64

// GlobItem matches paths using the well-known globbing mechanism.
type GlobItem struct {
	ID      int64
	Raw     string
	Pattern glob.Glob
	Count   int64
}

// NewGlobItem creates a new GlobItem from the given pattern
func NewGlobItem(id, cnt int64, pattern string) (Item, error) {
	var (
		err  error
		item = &GlobItem{ID: id, Count: cnt}
	)

	if item.Pattern, err = glob.Compile(pattern); err != nil {
		return nil, err
	}

	item.Raw = pattern
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

// GetID returns the Item's ID
func (i *GlobItem) GetID() int64 {
	return i.ID
} // func (i *GlobItem) GetID() int64

// GetPattern returns the Item's Pattern string
func (i *GlobItem) GetPattern() string {
	return i.Raw
} // func (i *GlobItem) GetPattern() string

// IsGlob returns true if the Item uses globbing (in this case: true)
func (i *GlobItem) IsGlob() bool { return true }

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
