// /home/krylon/go/src/github.com/blicero/snoopy/ui/search.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-23 16:57:49 krylon>

package ui

import (
	"fmt"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/model"
	"github.com/gotk3/gotk3/gtk"
)

func (g *SWin) handleSearchExec() {
	var (
		err       error
		qstr, msg string
		db        *database.Database
		files     []*model.File
	)

	if qstr, err = g.tabs[tiSearch].search.GetText(); err != nil {
		msg = fmt.Sprintf("Error getting search query: %s",
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
	}

	g.log.Printf("[DEBUG] Search for %q\n",
		qstr)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if files, err = db.MetaSearch(qstr); err != nil {
		msg = fmt.Sprintf("Error performing search for %q: %s",
			qstr,
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
	}

	var store = g.tabs[tiSearch].store.(*gtk.ListStore)

	store.Clear()

	for _, f := range files {
		var iter = store.Append()

		if err = store.Set(iter,
			[]int{0, 1, 2, 3, 4, 5},
			[]any{
				f.ID,
				f.Path,
				f.Type,
				f.LastRefresh.Format(common.TimestampFormatMinute),
				"???",
				f.CTime.Format(common.TimestampFormatMinute),
			},
		); err != nil {
			msg = fmt.Sprintf("Failed to set Iter: %s",
				err.Error())
			g.log.Printf("[ERROR] %s\n", msg)
			g.displayMsg(msg)
		}
	}
} // func (g *SWin) handleSearchExec()
