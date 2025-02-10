// /home/krylon/go/src/github.com/blicero/snoopy/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-10 21:01:32 krylon>

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/common/path"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/ui"
)

func main() {
	fmt.Printf("%s %s\n",
		common.AppName,
		common.Version)

	defer fmt.Println("Bye Bye")

	var (
		err   error
		base  string
		trunc bool
		win   *ui.SWin
	)

	flag.StringVar(&base, "base", common.BaseDir, "Directory to store application-related files in")
	flag.BoolVar(&trunc, "truncate", false, "Truncate the log file before starting")
	flag.Parse()

	defer func() {
		fmt.Printf("Database connections have waited for the lock %d times.\n",
			database.WaitCnt.Load())
	}()

	if base != common.BaseDir {
		if err = common.SetBaseDir(base); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"ERROR: %s\n",
				err.Error(),
			)
			os.Exit(1)
		}
	}

	if trunc {
		var logfile = common.Path(path.Log)
		if err = os.Truncate(logfile, 0); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Failed to truncate log file %s: %s\n",
				logfile,
				err.Error(),
			)
			os.Exit(1)
		}
	}

	if win, err = ui.Create(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to create GUI: %s\n",
			err.Error())
		os.Exit(1)
	}

	win.Run()
}
