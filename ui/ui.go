// /home/krylon/go/src/github.com/blicero/snoopy/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-04 19:32:14 krylon>

package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/extractor"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	"github.com/blicero/snoopy/walker"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

//go:generate stringer -type=MsgLevel

// MsgLevel indicates what to do with a Message
type MsgLevel uint8

// MsgStatusbar - display the message in the statusbar
// MsgDialog - display the message in a Dialog
// MsgLog - write the message to the Log
const (
	MsgStatusbar MsgLevel = iota
	MsgDialog
	MsgLog
)

// Msg is a message to be displayed in some form to the user.
// Gtk is not threadsafe, so I send Msg values through a channel, and a timeout
// in the GUI fetches and displays them regularly.
type Msg struct {
	Level   MsgLevel
	Message string
}

type tabContent struct {
	vbox   *gtk.Box
	sbox   *gtk.Box
	lbl    *gtk.Label
	search *gtk.Entry
	store  gtk.ITreeModel
	filter *gtk.TreeModelFilter
	view   *gtk.TreeView
	scr    *gtk.ScrolledWindow
}

// SWin is wraps up the UI main window, all of its contents, and all associated state.
type SWin struct {
	pool      *database.Pool
	scanner   *walker.Walker
	probe     *extractor.Extractor
	lock      sync.RWMutex // nolint: unused,deadcode,structcheck
	MsgQ      chan Msg
	log       *log.Logger
	win       *gtk.Window
	mainBox   *gtk.Box
	menu      *gtk.MenuBar
	notebook  *gtk.Notebook
	statusbar *gtk.Statusbar
	tabs      []tabContent
}

// Create creates a new UI instance and returns it.
func Create() (*SWin, error) {
	var (
		err error
		g   = &SWin{
			MsgQ: make(chan Msg, 5),
		}
	)

	if g.log, err = common.GetLogger(logdomain.GUI); err != nil {
		return nil, err
	} else if g.pool, err = database.NewPool(4); err != nil {
		g.log.Printf("[ERROR] Failed to create database connection pool: %s\n",
			err.Error())
		return nil, err
	} else if g.scanner, err = walker.New(); err != nil {
		g.log.Printf("[ERROR] Failed to create Walker: %s\n",
			err.Error())
		return nil, err
	} else if g.probe, err = extractor.New(); err != nil {
		g.log.Printf("[ERROR] Failed to create Extractor: %s\n",
			err.Error())
		return nil, err
	}

	if display := os.Getenv("DISPLAY"); display == "" {
		g.log.Println("[WARNING] Environment variable DISPLAY is not set. Trying :0.0")
		os.Setenv("DISPLAY", ":0.0")
	}

	gtk.Init(nil)

	if g.win, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL); err != nil {
		g.log.Printf("[ERROR] Cannot create Toplevel Window: %s\n",
			err.Error())
		return nil, err
	} else if g.mainBox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 1); err != nil {
		g.log.Printf("[ERROR] Cannot create Gtk Box: %s\n",
			err.Error())
		return nil, err
	} else if g.menu, err = gtk.MenuBarNew(); err != nil {
		g.log.Printf("[ERROR] Cannot create MenuBar: %s\n",
			err.Error())
		return nil, err
	} else if g.notebook, err = gtk.NotebookNew(); err != nil {
		g.log.Printf("[ERROR] Cannot create Notebook: %s\n",
			err.Error())
		return nil, err
	} else if g.statusbar, err = gtk.StatusbarNew(); err != nil {
		g.log.Printf("[ERROR] Cannot create Statusbar: %s\n",
			err.Error())
		return nil, err
	}

	g.tabs = make([]tabContent, len(viewList))
	for tIdx, v := range viewList {
		var (
			tab         tabContent
			lbl         *gtk.Label
			editHandler cellEditHandlerFactory
		)

		if tab.store, tab.filter, tab.view, err = v.create(editHandler); err != nil {
			g.log.Printf("[ERROR] Cannot create TreeView %q: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if tab.scr, err = gtk.ScrolledWindowNew(nil, nil); err != nil {
			g.log.Printf("[ERROR] Cannot create ScrolledWindow for %q: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if lbl, err = gtk.LabelNew(v.title); err != nil {
			g.log.Printf("[ERROR] Cannot create title Label for %q: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if tab.vbox, err = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 1); err != nil {
			g.log.Printf("[ERROR] Cannot create vbox for %q: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if tab.sbox, err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 1); err != nil {
			g.log.Printf("[ERROR] Cannot create sbox for %q: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if tab.lbl, err = gtk.LabelNew("Filter:"); err != nil {
			g.log.Printf("[ERROR] Cannot create Filter Label for %s: %s\n",
				v.title,
				err.Error())
			return nil, err
		} else if tab.search, err = gtk.EntryNew(); err != nil {
			g.log.Printf("[ERROR] Cannot create search entry for %s: %s\n",
				v.title,
				err.Error())
			return nil, err
		}

		tab.sbox.PackStart(tab.lbl, false, false, 1)
		tab.sbox.PackStart(tab.search, true, true, 1)
		tab.vbox.PackStart(tab.sbox, false, false, 1)
		tab.vbox.PackStart(tab.scr, true, true, 1)

		switch tabIdx(tIdx) {
		default:
			tab.filter.SetVisibleFunc(dummyFilter)
		}

		g.tabs[tIdx] = tab
		tab.scr.Add(tab.view)
		g.notebook.AppendPage(tab.vbox, lbl)
	}

	// Initialize views
	defer g.loadViewData()

	if err = g.initMenu(); err != nil {
		return nil, err
	}

	// Register signal handlers
	g.win.Connect("destroy", gtk.MainQuit)
	g.tabs[tiRoot].view.Connect("button-press-event", g.handleRootListClick)
	g.tabs[tiSearch].view.Connect("button-press-event", g.handleSearchResultClick)
	g.tabs[tiFiles].view.Connect("button-press-event", g.handleFileListClick)
	g.tabs[tiSearch].search.Connect("activate", g.handleSearchExec)

	g.win.Add(g.mainBox)
	g.mainBox.PackStart(g.menu, false, false, 0)
	g.mainBox.PackStart(g.notebook, true, true, 0)
	g.mainBox.PackStart(g.statusbar, false, false, 0)
	g.win.SetSizeRequest(960, 540)
	g.win.SetTitle(fmt.Sprintf("%s %s (built on %s)",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat)))
	g.win.ShowAll()

	return g, nil
} // func Create() (*SWin, error)

// Run executes gtk's main event loop.
func (g *SWin) Run() {
	go func() {
		var cnt = 0
		for {
			time.Sleep(time.Second * 5)
			cnt++
			var msg = Msg{
				Message: fmt.Sprintf("%s: Tick #%d",
					time.Now().Format(common.TimestampFormat),
					cnt),
				Level: MsgStatusbar,
			}
			g.MsgQ <- msg
			// g.statusbar.Push(666, msg)
		}
	}()

	glib.TimeoutAdd(1000, g.checkMsgQ)

	gtk.Main()
} // func (g *SWin) Run()

func (g *SWin) checkMsgQ() bool {
	select {
	case <-time.After(time.Millisecond * 50):
	case msg := <-g.MsgQ:
		switch msg.Level {
		case MsgStatusbar:
			g.log.Printf("[INFO] %s\n", msg.Message)
			g.statusbar.Push(666, msg.Message)
		case MsgDialog:
			g.log.Printf("[INFO] %s\n", msg.Message)
			g.displayMsg(msg.Message)
		case MsgLog:
			g.log.Printf("[DEBUG] %s\n", msg.Message)
		default:
			g.log.Printf("[ERROR] Unknown message type %s\n",
				msg.Level)
		}
	}

	return true
} // func (g *SWin) checkMsgQ() bool

// nolint: unused
func (g *SWin) displayMsg(msg string) {
	krylib.Trace()
	defer g.log.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err error
		dlg *gtk.Dialog
		lbl *gtk.Label
		box *gtk.Box
	)

	if dlg, err = gtk.DialogNewWithButtons(
		"Message",
		g.win,
		gtk.DIALOG_MODAL,
		[]interface{}{
			"Okay",
			gtk.RESPONSE_OK,
		},
	); err != nil {
		g.log.Printf("[ERROR] Cannot create dialog to display message: %s\nMesage would've been %q\n",
			err.Error(),
			msg)
		return
	}

	defer dlg.Close()

	if lbl, err = gtk.LabelNew(msg); err != nil {
		g.log.Printf("[ERROR] Cannot create label to display message: %s\nMessage would've been: %q\n",
			err.Error(),
			msg)
		return
	} else if box, err = dlg.GetContentArea(); err != nil {
		g.log.Printf("[ERROR] Cannot get ContentArea of Dialog to display message: %s\nMessage would've been %q\n",
			err.Error(),
			msg)
		return
	}

	box.PackStart(lbl, true, true, 0)
	dlg.ShowAll()
	dlg.Run()
} // func (g *SWin) displayMsg(msg string)

func (g *SWin) logError(msg string) {
	g.log.Printf("[ERROR] %s\n", msg)
	g.displayMsg(msg)
} // func (g *SWin) logError(msg string)

func (g *SWin) quit() {
	gtk.MainQuit()
} // func (g *SWin) quit()

func (g *SWin) handleAddRoot() {
	var (
		err error
		dlg *gtk.FileChooserDialog
		res gtk.ResponseType
	)

	if dlg, err = gtk.FileChooserDialogNewWith2Buttons(
		"Add Directory Tree",
		g.win,
		gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"Cancel",
		gtk.RESPONSE_CANCEL,
		"OK",
		gtk.RESPONSE_OK,
	); err != nil {
		g.log.Printf("[ERROR] Cannot create FileChooserDialog: %s\n",
			err.Error())
		return
	}

	defer dlg.Close()

	res = dlg.Run()

	switch res {
	case gtk.RESPONSE_CANCEL:
		g.log.Println("[TRACE] Suit yourself")
		return
	case gtk.RESPONSE_OK:
		var (
			db       *database.Database
			store    *gtk.ListStore
			iter     *gtk.TreeIter
			stampStr string
			root     = new(model.Root)
		)

		if root.Path, err = dlg.GetCurrentFolder(); err != nil {
			g.log.Printf("[ERROR] Cannot get folder from Dialog: %s\n",
				err.Error())
			return
		}

		g.log.Printf("[DEBUG] Adding Root folder %s to database\n",
			root.Path)

		db = g.pool.Get()
		defer g.pool.Put(db)

		if err = db.RootAdd(root); err != nil {
			g.log.Printf("[ERROR] Failed to add root folder %s to Database: %s\n",
				root.Path,
				err.Error())
			return
		}

		store = g.tabs[tiRoot].store.(*gtk.ListStore)
		iter = store.Append()

		stampStr = root.LastScan.Format(common.TimestampFormatMinute)

		if err = store.SetValue(iter, 0, root.ID); err != nil {
			g.log.Printf("[ERROR] Cannot set ID for Root %s (%d): %s\n",
				root.Path,
				root.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 1, root.Path); err != nil {
			g.log.Printf("[ERROR] Cannot display Path for Root %s: %s\n",
				root.Path,
				err.Error())
			return
		} else if err = store.SetValue(iter, 2, stampStr); err != nil {
			g.log.Printf("[ERROR] Cannot display LastScan timestamp of Root %s: %s\n",
				root.Path,
				err.Error())
			return
		}
	default:
		g.log.Printf("[ERROR] Unknown dialog response type: %d\n",
			res)
	}
} // func (g *SWin) handleAddRoot()

func (g *SWin) handleScanRoot() {
	krylib.Trace()

	var (
		err   error
		db    *database.Database
		roots []*model.Root
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if roots, err = db.RootGetAll(); err != nil {
		g.log.Printf("[ERROR] Failed to load all Roots from database: %s\n",
			err.Error())
	}

	for idx, r := range roots {
		g.log.Printf("[INFO] Schedule root %d/%d (%s) for scan\n",
			idx+1,
			len(roots),
			r.Path)
		g.scanner.ScheduleScan(r)
	}
} // func (g *SWin) handleScanRoot()

func (g *SWin) handleExtractorRun() {
	defer g.log.Println("[DEBUG] handleExtractorRun() exitting")
	if g.probe.IsActive() {
		g.displayMsg("Extractor is already active")
		return
	}

	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				g.log.Printf("[CRITICAL] Panic in Extractor: %#v\n", ex)
			}
		}()
		// defer g.displayMsg("Extractor is finished")
		g.probe.Run()
		g.log.Println("[INFO] Extractor is finished")
	}()
} // func (g *SWin) handleExtractorRun()

func (g *SWin) loadViewData() {
	var (
		err     error
		db      *database.Database
		store   *gtk.ListStore
		roots   []*model.Root
		files   []*model.File
		blItems []blacklist.Item
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	/////////////////////////////////
	// Roots ////////////////////////
	/////////////////////////////////

	if roots, err = db.RootGetAll(); err != nil {
		g.log.Printf("[ERROR] Failed to load roots from database: %s\n",
			err.Error())
		return
	}

	store = g.tabs[tiRoot].store.(*gtk.ListStore)

	store.Clear()

	for _, r := range roots {
		var (
			stampStr string
			iter     = store.Append()
		)

		stampStr = r.LastScan.Format(common.TimestampFormatMinute)

		if err = store.SetValue(iter, 0, r.ID); err != nil {
			g.log.Printf("[ERROR] Cannot set ID for Root %s (%d): %s\n",
				r.Path,
				r.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 1, r.Path); err != nil {
			g.log.Printf("[ERROR] Cannot display Path for Root %s: %s\n",
				r.Path,
				err.Error())
			return
		} else if err = store.SetValue(iter, 2, stampStr); err != nil {
			g.log.Printf("[ERROR] Cannot display LastScan timestamp of Root %s: %s\n",
				r.Path,
				err.Error())
			return
		}
	}

	/////////////////////////////////
	// Files ////////////////////////
	/////////////////////////////////

	store = g.tabs[tiFiles].store.(*gtk.ListStore)

	if files, err = db.FileGetAll(); err != nil {
		g.log.Printf("[ERROR] Failed to load all Files from database: %s\n",
			err.Error())
		return
	}

	store.Clear()

	for _, f := range files {
		var (
			stampStr = f.LastRefresh.Format(common.TimestampFormatMinute)
			ctimeStr = f.CTime.Format(common.TimestampFormatMinute)
			iter     = store.Append()
		)

		if err = store.SetValue(iter, 0, f.ID); err != nil {
			g.log.Printf("[ERROR] Cannot set ID for File %s (%d): %s\n",
				f.Path,
				f.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 1, f.Path); err != nil {
			g.log.Printf("[ERROR] Cannot set Path for File %s (%d): %s\n",
				f.Path,
				f.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 2, f.Type); err != nil {
			g.log.Printf("[ERROR] Cannot set MIME Type for File %s (%d): %s\n",
				f.Path,
				f.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 3, stampStr); err != nil {
			g.log.Printf("[ERROR] Cannot set LastRefresh for File %s (%d): %s\n",
				f.Path,
				f.ID,
				err.Error())
			return
		} else if err = store.SetValue(iter, 3, ctimeStr); err != nil {
			g.log.Printf("[ERROR] Cannot set CTime for File %s (%d): %s\n",
				f.Path,
				f.ID,
				err.Error())
			return
		}
	}

	/////////////////////////////////
	// Blacklist ////////////////////
	/////////////////////////////////

	if blItems, err = db.BlacklistGetAll(); err != nil {
		var msg = fmt.Sprintf("Failed to load BlacklistItems: %s",
			err.Error())
		g.logError(msg)
		return
	}

	store = g.tabs[tiBlacklist].store.(*gtk.ListStore)
	store.Clear()

	for _, i := range blItems {
		var iter = store.Append()
		if err = store.SetValue(iter, 0, i.GetID()); err != nil {
			g.logError(err.Error())
			return
		} else if err = store.SetValue(iter, 1, i.GetPattern()); err != nil {
			g.logError(err.Error())
			return
		} else if err = store.SetValue(iter, 2, i.IsGlob()); err != nil {
			g.logError(err.Error())
			return
		} else if err = store.SetValue(iter, 3, i.HitCount()); err != nil {
			g.logError(err.Error())
			return
		}
	}

} // func (g *SWin) loadViewData()

func (g *SWin) handleRootListClick(view *gtk.TreeView, evt *gdk.Event) {
	krylib.Trace()
	var be = gdk.EventButtonNewFromEvent(evt)

	if be.Button() != gdk.BUTTON_SECONDARY {
		return
	}

	var (
		err    error
		exists bool
		x, y   float64
		msg    string
		path   *gtk.TreePath
		store  *gtk.TreeModel
		istore gtk.ITreeModel
		filter *gtk.TreeModelFilter
		iter   *gtk.TreeIter
	)

	x = be.X()
	y = be.Y()
	path, _, _, _, exists = view.GetPathAtPos(int(x), int(y))

	if !exists {
		g.log.Printf("[DEBUG] There is no item at %f/%f\n",
			x,
			y)
		return
	} else if istore, err = view.GetModel(); err != nil {
		g.log.Printf("[ERROR] Cannot get Model from View: %s\n",
			err.Error())
		return
	}

	filter = istore.(*gtk.TreeModelFilter)
	store = g.tabs[tiRoot].store.ToTreeModel()

	if iter, err = filter.GetIter(path); err != nil {
		g.log.Printf("[ERROR] Cannot get Iter from TreePath %s: %s\n",
			path,
			err.Error())
		return
	}

	iter = filter.ConvertIterToChildIter(iter)
	path, _ = store.GetPath(iter)

	var (
		val *glib.Value
		gv  any
		id  int64
	)

	if val, err = store.GetValue(iter, 0); err != nil {
		g.log.Printf("[ERROR] Cannot get value for column 0: %s\n",
			err.Error())
		return
	} else if gv, err = val.GoValue(); err != nil {
		g.log.Printf("[ERROR] Cannot get Go value from GLib value: %s\n",
			err.Error())
		return
	}

	switch v := gv.(type) {
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		g.log.Printf("[ERROR] Unexpected type for ID column: %T\n",
			v)
		return
	}

	var (
		db   *database.Database
		root *model.Root
		menu *gtk.Menu
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if root, err = db.RootGetByID(id); err != nil {
		msg = fmt.Sprintf("Failed to load Root #%d: %s",
			id,
			err.Error())
		g.logError(msg)
		return
	} else if root == nil {
		msg = fmt.Sprintf("Root #%d was not found in database", id)
		g.log.Printf("[CANTHAPPEN] %s\n", msg)
		g.displayMsg(msg)
		return
	} else if menu, err = g.mkContextMenuRoot(path, root); err != nil {
		g.logError(err.Error())
		return
	}

	menu.ShowAll()
	menu.PopupAtPointer(evt)
} // func (g *SWin) handleRootListClick(view *gtk.TreeView, evt *gdk.Event)

func (g *SWin) mkContextMenuRoot(_ *gtk.TreePath, root *model.Root) (*gtk.Menu, error) {
	krylib.Trace()
	var (
		err                  error
		menu                 *gtk.Menu
		scanItem, deleteItem *gtk.MenuItem
	)

	if menu, err = gtk.MenuNew(); err != nil {
		goto ERROR
	} else if scanItem, err = gtk.MenuItemNewWithMnemonic("_Scan"); err != nil {
		goto ERROR
	} else if deleteItem, err = gtk.MenuItemNewWithMnemonic("_Delete"); err != nil {
		goto ERROR
	}

	scanItem.Connect("activate", func() { g.scanner.ScheduleScan(root) })
	deleteItem.Connect("activate", func() { g.displayMsg("IMPLEMENT ME!") })

	menu.Append(scanItem)
	menu.Append(deleteItem)
	return menu, nil

ERROR:
	g.logError(err.Error())
	return nil, err
} // func (g *SWin) mkContextMenuRoot(path *gtk.TreePath, root *model.Root) (*gtk.Menu, error)

func (g *SWin) handlePrune() {
	krylib.Trace()

	var (
		err    error
		db     *database.Database
		files  []*model.File
		status bool
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if err = db.Begin(); err != nil {
		g.log.Printf("[ERROR] Failed begin transaction: %s\n",
			err.Error())
		return
	}

	defer func() {
		var (
			ex  error
			msg string
		)

		if status {
			if ex = db.Commit(); ex != nil {
				msg = fmt.Sprintf("Failed to commit database transaction: %s",
					ex.Error())
				g.MsgQ <- Msg{Level: MsgDialog, Message: msg}
			}
		} else {
			if ex = db.Rollback(); ex != nil {
				msg = fmt.Sprintf("Failed to roll back database transaction: %s",
					ex.Error())
				g.MsgQ <- Msg{Level: MsgDialog, Message: msg}
			}
		}
	}()

	if files, err = db.FileGetAll(); err != nil {
		g.MsgQ <- Msg{
			Level: MsgDialog,
			Message: fmt.Sprintf("Failed to get all Files from database: %s",
				err.Error()),
		}
		return
	}

	var cnt = 0

	for _, f := range files {
		var exist bool
		if exist, err = krylib.Fexists(f.Path); err != nil {
			g.log.Printf("[ERROR] Cannot check if File %s exists: %s\n",
				f.Path,
				err.Error())
			continue
		} else if !exist {
			g.log.Printf("[DEBUG] Remove %s from database\n",
				f.Path)
			if err = db.FileDelete(f); err != nil {
				var m = Msg{
					Message: fmt.Sprintf("Failed to delete File %s from database: %s\n",
						f.Path,
						err.Error()),
					Level: MsgDialog,
				}
				g.MsgQ <- m
				return
			}

			cnt++
		}
	}

	g.log.Printf("[INFO] Removed %d Files from database.\n", cnt)
	status = true
} // func (g *SWin) handlePrune()

func (g *SWin) handleSearchResultClick(view *gtk.TreeView, ev *gdk.Event) {
	krylib.Trace()
	var be = gdk.EventButtonNewFromEvent(ev)

	if be.Button() != gdk.BUTTON_SECONDARY {
		return
	}

	var (
		err    error
		exists bool
		x, y   float64
		msg    string
		path   *gtk.TreePath
		store  *gtk.TreeModel
		istore gtk.ITreeModel
		filter *gtk.TreeModelFilter
		iter   *gtk.TreeIter
	)

	x = be.X()
	y = be.Y()
	path, _, _, _, exists = view.GetPathAtPos(int(x), int(y))

	if !exists {
		g.log.Printf("[DEBUG] There is no item at %f/%f\n",
			x,
			y)
		return
	} else if istore, err = view.GetModel(); err != nil {
		g.log.Printf("[ERROR] Cannot get Model from View: %s\n",
			err.Error())
		return
	}

	filter = istore.(*gtk.TreeModelFilter)
	store = g.tabs[tiSearch].store.ToTreeModel()

	if iter, err = filter.GetIter(path); err != nil {
		g.log.Printf("[ERROR] Cannot get Iter from TreePath %s: %s\n",
			path,
			err.Error())
		return
	}

	iter = filter.ConvertIterToChildIter(iter)
	path, _ = store.GetPath(iter) // nolint: ineffassign,staticcheck

	var (
		val *glib.Value
		gv  any
		id  int64
	)
	if val, err = store.GetValue(iter, 0); err != nil {
		g.log.Printf("[ERROR] Cannot get value for column 0: %s\n",
			err.Error())
		return
	} else if gv, err = val.GoValue(); err != nil {
		g.log.Printf("[ERROR] Cannot get Go value from GLib value: %s\n",
			err.Error())
		return
	}

	switch v := gv.(type) {
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		g.log.Printf("[ERROR] Unexpected type for ID column: %T\n",
			v)
		return
	}

	var (
		db   *database.Database
		f    *model.File
		menu *gtk.Menu
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if f, err = db.FileGetByID(id); err != nil {
		msg = fmt.Sprintf("Failed to load File #%d from Database: %s",
			id,
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
		return
	} else if f == nil {
		msg = fmt.Sprintf("File %d was not found in Database", id)
		g.log.Printf("[CANTHAPPEN] %s\n", msg)
		g.displayMsg(msg)
		return
	} else if menu, err = g.mkContextMenuFile(nil, f); err != nil {
		msg = fmt.Sprintf("Failed to create context menu for %s: %s",
			f.Path,
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
		return
	}

	menu.ShowAll()
	menu.PopupAtPointer(ev)
} // func (g *SWin) handleSearchResultClick(view *gtk.TreeView, ev *gdk.Event)

func (g *SWin) handleFileListClick(view *gtk.TreeView, ev *gdk.Event) {
	krylib.Trace()
	var be = gdk.EventButtonNewFromEvent(ev)

	if be.Button() != gdk.BUTTON_SECONDARY {
		return
	}

	var (
		err    error
		exists bool
		x, y   float64
		msg    string
		path   *gtk.TreePath
		store  *gtk.TreeModel
		istore gtk.ITreeModel
		filter *gtk.TreeModelFilter
		iter   *gtk.TreeIter
	)

	x = be.X()
	y = be.Y()
	path, _, _, _, exists = view.GetPathAtPos(int(x), int(y))

	if !exists {
		g.log.Printf("[DEBUG] There is no item at %f/%f\n",
			x,
			y)
		return
	} else if istore, err = view.GetModel(); err != nil {
		g.log.Printf("[ERROR] Cannot get Model from View: %s\n",
			err.Error())
		return
	}

	filter = istore.(*gtk.TreeModelFilter)
	store = g.tabs[tiFiles].store.ToTreeModel()

	if iter, err = filter.GetIter(path); err != nil {
		g.log.Printf("[ERROR] Cannot get Iter from TreePath %s: %s\n",
			path,
			err.Error())
		return
	}

	iter = filter.ConvertIterToChildIter(iter)
	path, _ = store.GetPath(iter) // nolint: ineffassign,staticcheck

	var (
		val *glib.Value
		gv  any
		id  int64
	)
	if val, err = store.GetValue(iter, 0); err != nil {
		g.log.Printf("[ERROR] Cannot get value for column 0: %s\n",
			err.Error())
		return
	} else if gv, err = val.GoValue(); err != nil {
		g.log.Printf("[ERROR] Cannot get Go value from GLib value: %s\n",
			err.Error())
		return
	}

	switch v := gv.(type) {
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		g.log.Printf("[ERROR] Unexpected type for ID column: %T\n",
			v)
		return
	}

	var (
		db   *database.Database
		f    *model.File
		menu *gtk.Menu
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if f, err = db.FileGetByID(id); err != nil {
		msg = fmt.Sprintf("Failed to load File #%d from Database: %s",
			id,
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
		return
	} else if f == nil {
		msg = fmt.Sprintf("File %d was not found in Database", id)
		g.log.Printf("[CANTHAPPEN] %s\n", msg)
		g.displayMsg(msg)
		return
	} else if menu, err = g.mkContextMenuFile(nil, f); err != nil {
		msg = fmt.Sprintf("Failed to create context menu for %s: %s",
			f.Path,
			err.Error())
		g.log.Printf("[ERROR] %s\n", msg)
		g.displayMsg(msg)
		return
	}

	menu.ShowAll()
	menu.PopupAtPointer(ev)
} // func (g *SWin) handleFileListClick(view *gtk.TreeView, ev *gdk.Event)

func (g *SWin) mkContextMenuFile(_ *gtk.TreePath, f *model.File) (*gtk.Menu, error) {
	var (
		err                error
		db                 *database.Database
		menu               *gtk.Menu
		openItem, infoItem *gtk.MenuItem
	)

	db = g.pool.Get()
	defer g.pool.Put(db)

	if menu, err = gtk.MenuNew(); err != nil {
		goto ERROR
	} else if openItem, err = gtk.MenuItemNewWithMnemonic("_Open"); err != nil {
		goto ERROR
	} else if infoItem, err = gtk.MenuItemNewWithMnemonic("_Info"); err != nil {
		goto ERROR
	}

	openItem.Connect("activate", func() { g.handleOpenFile(f) })
	infoItem.Connect("activate", func() { g.handleFileInfo(f) })

	menu.Append(openItem)
	menu.Append(infoItem)

	return menu, nil

ERROR:
	g.logError(err.Error())
	return nil, err
} // func (g *Swin) mkContextMenuFile(p *gtk.TreePath, f *model.File) (*gtk.Menu, error)

func (g *SWin) handleOpenFile(f *model.File) {
	const prog = "xdg-open"
	var (
		err error
		cmd *exec.Cmd
	)

	cmd = exec.Command(prog, f.Path)

	if err = cmd.Run(); err != nil {
		g.MsgQ <- Msg{
			Level: MsgDialog,
			Message: fmt.Sprintf("Failed to open File %s: %s",
				f.Path,
				err.Error()),
		}
	}
} // func (g *SWin) handleOpenFile(f *model.File)

func (g *SWin) handleFileInfo(f *model.File) {
	g.log.Printf("[DEBUG] IMPLEMENTME: handleFileInfo(%s)\n",
		f.Path)
} // func (g *SWin) handleFileInfo(f *model.File)
