// /home/krylon/go/src/github.com/blicero/snoopy/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-02 20:36:00 krylon>

package main

import (
	"fmt"
	"os"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/ui"
)

func main() {
	fmt.Printf("%s %s\n",
		common.AppName,
		common.Version)

	defer fmt.Println("Bye Bye")

	var (
		err error
		win *ui.SWin
	)

	if win, err = ui.Create(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to create GUI: %s\n",
			err.Error())
		os.Exit(1)
	}

	win.Run()
}
