// /home/krylon/go/src/github.com/blicero/snoopy/ui/view.go
// -*- mode: go; coding: utf-8; -*-
// Created on 02. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-07 18:25:28 krylon>

package ui

import (
	"fmt"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type tabIdx uint8

// nolint: deadcode,unused,varcheck
const (
	tiRoot tabIdx = iota
	tiFiles
	tiSearch
	tiBlacklist
)

type storeType uint8

const (
	storeList storeType = iota
	storeTree
)

type column struct {
	colType glib.Type
	title   string
	edit    bool
}

func createCol(title string, id int) (*gtk.TreeViewColumn, *gtk.CellRendererText, error) {
	// krylib.Trace()
	// defer fmt.Printf("[TRACE] EXIT %s\n",
	// 	krylib.TraceInfo())

	renderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, err
	}

	col, err := gtk.TreeViewColumnNewWithAttribute(title, renderer, "text", id)
	if err != nil {
		return nil, nil, err
	}

	col.SetResizable(true)

	return col, renderer, nil
} // func createCol(title string, id int) (*gtk.TreeViewColumn, *gtk.CellRendererText, error)

type cellEditHandlerFactory func(int) func(*gtk.CellRendererText, string, string)

type view struct {
	title   string
	store   storeType
	columns []column
}

func (v *view) typeList() []glib.Type {
	var res = make([]glib.Type, len(v.columns))

	for i, c := range v.columns {
		res[i] = c.colType
	}

	return res
} // func (v *view) typeList() []glib.Type

func (v *view) create(handlerFactory cellEditHandlerFactory) (gtk.ITreeModel, *gtk.TreeModelFilter, *gtk.TreeView, error) {
	var (
		err    error
		cols   []glib.Type
		store  gtk.ITreeModel
		filter *gtk.TreeModelFilter
		tv     *gtk.TreeView
	)

	cols = v.typeList()
	switch v.store {
	case storeList:
		if store, err = gtk.ListStoreNew(cols...); err != nil {
			return nil, nil, nil, err
		}
	case storeTree:
		if store, err = gtk.TreeStoreNew(cols...); err != nil {
			return nil, nil, nil, err
		}
	default:
		err = fmt.Errorf("invalid Store type %d", v.store)
		return nil, nil, nil, err
	}

	if filter, err = store.ToTreeModel().FilterNew(nil); err != nil {
		return nil, nil, nil, err
	} else if tv, err = gtk.TreeViewNewWithModel(filter); err != nil {
		return nil, nil, nil, err
	}

	//filter.SetVisibleFunc(dummyFilter)

	for idx, cSpec := range v.columns {
		var (
			col      *gtk.TreeViewColumn
			renderer *gtk.CellRendererText
		)

		if col, renderer, err = createCol(cSpec.title, idx); err != nil {
			return nil, nil, nil, err
		}

		renderer.Set("editable", cSpec.edit)     // nolint: errcheck
		renderer.Set("editable-set", cSpec.edit) // nolint: errcheck
		if cSpec.edit && handlerFactory != nil {
			renderer.Connect("edited", handlerFactory(idx))
		}

		tv.AppendColumn(col)
	}

	return store, filter, tv, nil
} // func (v *view) create(handlerFactory cellEditHandlerFactory) (gtk.ITreeModel, *gtk.TreeModelFilter, *gtk.TreeView, error)

func dummyFilter(model *gtk.TreeModel, iter *gtk.TreeIter) bool {
	return true
} // func dummyFilter(model *gtk.TreeModel, iter *gtk.TreeIter) bool

var viewList = []view{
	{
		title: "Root",
		store: storeList,
		columns: []column{
			{
				colType: glib.TYPE_INT,
				title:   "ID",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Path",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Last Scan",
			},
		},
	},
	{
		title: "Files",
		store: storeList,
		columns: []column{
			{
				colType: glib.TYPE_INT,
				title:   "ID",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Path",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Type",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Last Refresh",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Size",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "CTime",
			},
		},
	},
	{
		title: "Search",
		store: storeList,
		columns: []column{
			{
				colType: glib.TYPE_INT,
				title:   "ID",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Path",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Type",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Last Refresh",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Size",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "CTime",
			},
		},
	},
	{
		title: "Blacklist",
		store: storeList,
		columns: []column{
			{
				colType: glib.TYPE_INT,
				title:   "ID",
			},
			{
				colType: glib.TYPE_STRING,
				title:   "Pattern",
				edit:    true,
			},
			{
				colType: glib.TYPE_BOOLEAN,
				title:   "Glob?",
			},
			{
				colType: glib.TYPE_INT,
				title:   "# Hits",
			},
		},
	},
}
