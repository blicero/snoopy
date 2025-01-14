// /home/krylon/go/src/github.com/blicero/snoopy/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-13 18:12:13 krylon>

package ui

import (
	"fmt"
	"log"
	"os"
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

// Swin is wraps up the UI main window, all of its contents, and all associated state.
type SWin struct {
	pool      *database.Pool
	scanner   *walker.Walker
	probe     *extractor.Extractor
	lock      sync.RWMutex // nolint: unused,deadcode,structcheck
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
		g   = new(SWin)
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
			time.Sleep(time.Second)
			cnt++
			var msg = fmt.Sprintf("%s: Tick #%d",
				time.Now().Format(common.TimestampFormat),
				cnt)
			g.statusbar.Push(666, msg)
		}
	}()

	gtk.Main()
} // func (g *SWin) Run()

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

// func (g *SWin) scanFolder() {
// 	krylib.Trace()
// 	defer g.log.Printf("[TRACE] EXIT %s\n",
// 		krylib.TraceInfo())

// 	var (
// 		err error
// 		dlg *gtk.FileChooserDialog
// 		res gtk.ResponseType
// 	)

// 	if dlg, err = gtk.FileChooserDialogNewWith2Buttons(
// 		"Scan Folder",
// 		g.win,
// 		gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER,
// 		"Cancel",
// 		gtk.RESPONSE_CANCEL,
// 		"OK",
// 		gtk.RESPONSE_OK,
// 	); err != nil {
// 		g.log.Printf("[ERROR] Cannot create FileChooserDialog: %s\n",
// 			err.Error())
// 		return
// 	}

// 	defer dlg.Close()

// 	res = dlg.Run()

// 	switch res {
// 	case gtk.RESPONSE_CANCEL:
// 		g.log.Println("[DEBUG] Ha, you almost got me.")
// 		return
// 	case gtk.RESPONSE_OK:
// 		var path string
// 		if path, err = dlg.GetCurrentFolder(); err != nil {
// 			g.log.Printf("[ERROR] Cannot get folder from dialog: %s\n",
// 				err.Error())
// 			return
// 		}

// 		go g.scanner.Walk(path) // nolint: errcheck
// 		glib.TimeoutAdd(1000,
// 			func() bool {
// 				var (
// 					ex   error
// 					item *gtk.MenuItem
// 				)

// 				if item, ex = gtk.MenuItemNewWithLabel(path); ex != nil {
// 					g.log.Printf("[ERROR] Cannot create MenuItem for %q: %s\n",
// 						path,
// 						ex.Error())
// 					return false
// 				}

// 				item.Connect("activate", func() {
// 					g.statusbar.Push(666, fmt.Sprintf("Update %s", path))
// 					go g.scanner.Walk(path) // nolint: errcheck
// 				})

// 				g.dMenu.Append(item)

// 				return false
// 			})
// 	}
// } // func (g *SWin) scanFolder()

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
	} else if menu, err = g.mkRootContextMenu(path, root); err != nil {
		g.logError(err.Error())
		return
	}

	menu.ShowAll()
	menu.PopupAtPointer(evt)
} // func (g *SWin) handleRootListClick(view *gtk.TreeView, evt *gdk.Event)

func (g *SWin) mkRootContextMenu(_ *gtk.TreePath, root *model.Root) (*gtk.Menu, error) {
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
} // func (g *SWin) mkRootContextMenu(path *gtk.TreePath, root *model.Root) (*gtk.Menu, error)

func (g *SWin) lookForMetadata() {
	if g.probe.IsActive() {
		g.displayMsg("Extractor is already active")
		return
	}

	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				g.logError(fmt.Sprintf("Panic in Extractor: %#v\n", ex))
			}
		}()
		defer g.displayMsg("Extractor is finished")
		g.probe.Run()
	}()
} // func (g *SWin) lookForMetadata()
