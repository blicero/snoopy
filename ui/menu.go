// /home/krylon/go/src/github.com/blicero/snoopy/ui/menu.go
// -*- mode: go; coding: utf-8; -*-
// Created on 31. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-07 18:36:11 krylon>

package ui

import (
	"github.com/blicero/krylib"
	"github.com/gotk3/gotk3/gtk"
)

func (g *SWin) initMenu() error {
	krylib.Trace()
	defer g.log.Printf("[TRACE] EXIT %s\n",
		krylib.TraceInfo())

	var (
		err                                 error
		fileMenu, editMenu                  *gtk.Menu
		fmItem, emItem                      *gtk.MenuItem
		rootAddItem, rootScanItem, quitItem *gtk.MenuItem
		prefItem, loadViewItem              *gtk.MenuItem
	)

	// Step 1 Create the menus and items

	if fileMenu, err = gtk.MenuNew(); err != nil {
		g.log.Printf("[ERROR] Failed to create File Menu: %s\n",
			err.Error())
		return err
	} else if editMenu, err = gtk.MenuNew(); err != nil {
		g.log.Printf("[ERROR] Failed to create Edit Menu: %s\n",
			err.Error())
		return err
	} else if fmItem, err = gtk.MenuItemNewWithMnemonic("_File"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu Item _File: %s\n",
			err.Error())
		return err
	} else if emItem, err = gtk.MenuItemNewWithMnemonic("_Edit"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu Item _Edit: %s\n",
			err.Error())
		return err
	} else if rootAddItem, err = gtk.MenuItemNewWithMnemonic("_Add Root..."); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu item to add root: %s\n",
			err.Error())
		return err
	} else if rootScanItem, err = gtk.MenuItemNewWithMnemonic("_Scan Roots"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu item to scan roots: %s\n",
			err.Error())
		return err
	} else if quitItem, err = gtk.MenuItemNewWithMnemonic("_Quit"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu item to quit: %s\n",
			err.Error())
		return err
	} else if prefItem, err = gtk.MenuItemNewWithMnemonic("_Preferences"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu Item _Preferences: %s\n",
			err.Error())
		return err
	} else if loadViewItem, err = gtk.MenuItemNewWithMnemonic("_Load View data"); err != nil {
		g.log.Printf("[ERROR] Failed to create Menu Item _Load View data: %s\n",
			err.Error())
		return err
	}

	// Step 2 Assemble the menus and add them to the menubar

	fmItem.SetSubmenu(fileMenu)
	fileMenu.Append(rootAddItem)
	fileMenu.Append(rootScanItem)
	fileMenu.Append(quitItem)

	emItem.SetSubmenu(editMenu)
	editMenu.Append(loadViewItem)
	editMenu.Append(prefItem)

	g.menu.Append(fmItem)
	g.menu.Append(emItem)

	// Step 3 Register signal handlers
	quitItem.Connect("activate", g.quit)
	rootAddItem.Connect("activate", g.handleAddRoot)
	rootScanItem.Connect("activate", g.handleScanRoot)

	loadViewItem.Connect("activate", g.loadViewData)

	return nil
} // func (g *SWin) initMenu() error
