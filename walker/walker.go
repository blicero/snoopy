// /home/krylon/go/src/github.com/blicero/snoopy/walker/walker.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-23 20:30:17 krylon>

// Package walker implements the traversal of directories and the processing
// of the files therein.
package walker

import (
	"fmt"
	"log"
	"os"

	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/logdomain"
)

// Walker traverses directory trees and processes the files it finds.
type Walker struct {
	log *log.Logger
	bl  *blacklist.Blacklist
}

// NewWalker creates a new Walker instance that uses the given Blacklist.
func NewWalker(bl *blacklist.Blacklist) (*Walker, error) {
	var (
		err error
		w   = &Walker{bl: bl}
	)

	if w.log, err = common.GetLogger(logdomain.Walker); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Log for Walker: %s\n",
			err.Error(),
		)
		return nil, err
	}

	return w, nil
} // func NewWalker(bl *blacklist.Blacklist) (*Walker, error)
