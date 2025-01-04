// /home/krylon/go/src/github.com/blicero/snoopy/ui/ui.go
// -*- mode: go; coding: utf-8; -*-
// Created on 30. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-04 14:42:20 krylon>

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
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	"github.com/blicero/snoopy/walker"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type column struct {
	colType glib.Type
	title   string
	edit    bool
}

// nolint: unused,deadcode
var cols = []column{
	{
		colType: glib.TYPE_INT,
		title:   "Root ID",
	},
	{
		colType: glib.TYPE_STRING,
		title:   "Root Path",
	},
	{
		colType: glib.TYPE_INT,
		title:   "File ID",
	},
	{
		colType: glib.TYPE_STRING,
		title:   "Path",
	},
	{
		colType: glib.TYPE_STRING,
		title:   "Type",
		edit:    true,
	},
	{
		colType: glib.TYPE_STRING,
		title:   "Size",
	},
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
	} else if g.scanner, err = walker.NewWalker(blacklist.NewBlacklist()); err != nil {
		g.log.Printf("[ERROR] Failed to create Walker: %s\n",
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

	if err = g.initMenu(); err != nil {
		return nil, err
	}

	// ...

	g.win.Connect("destroy", gtk.MainQuit)

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

func dummyFilter(model *gtk.TreeModel, iter *gtk.TreeIter) bool {
	return true
} // func dummyFilter(model *gtk.TreeModel, iter *gtk.TreeIter) bool

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
