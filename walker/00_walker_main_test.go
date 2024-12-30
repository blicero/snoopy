// /home/krylon/go/src/github.com/blicero/snoopy/walker/00_walker_main_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 28. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-30 05:48:54 krylon>

package walker

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blicero/snoopy/common"
)

const (
	testRootWidth   = 8
	testRootDepth   = 3
	testRootFileCnt = 32
)

var (
	testRoot string
	w        *Walker
	fileCnt  int
)

func TestMain(m *testing.M) {
	var (
		err     error
		result  int
		baseDir = time.Now().Format("/tmp/snoopy_walker_test_20060102_150405")
	)

	if err = common.SetBaseDir(baseDir); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Cannot set BaseDir to %s: %s\n",
			baseDir,
			err.Error())
		os.Exit(1)
	} else if testRoot, err = initTestRoot(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to initialize test root directory: %s\n",
			err.Error())
		os.Exit(1)
	} else if result = m.Run(); result == 0 {
		// If any test failed, we keep the test directory (and the
		// database inside it) around, so we can manually inspect it
		// if needed.
		// If all tests pass, OTOH, we can safely remove the directory.
		// fmt.Printf("NOT Removing BaseDir %s\n",
		// 	baseDir)
		_ = os.RemoveAll(baseDir)
		_ = os.RemoveAll(testRoot)
	} else {
		fmt.Printf(">>> TEST DIRECTORY: %s\n", baseDir)
	}

	os.Exit(result)
}

func initTestRoot() (string, error) {
	var (
		err  error
		path string
	)

	if path, err = os.MkdirTemp("", "snoopy_walker_test_root"); err != nil {
		return "", err
	} else if err = populateTestRoot(path, testRootDepth); err != nil {
		return "", err
	}

	fmt.Fprintf(
		os.Stderr,
		"Created test root at %s\n",
		path)

	return path, nil
} // func initTestRoot() (string, error)

func populateTestRoot(path string, depth int) error {
	var (
		err error
		cnt = rand.Intn(testRootFileCnt) + 1
	)

	if err = os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	// Generate some files
	for i := 0; i < cnt; i++ {
		var (
			fh    *os.File
			fname = filepath.Join(path, randomFilename(i+1))
		)

		if fh, err = os.Create(fname); err != nil {
			return err
		} else if err = fh.Close(); err != nil {
			return err
		}

		fileCnt++
	}

	if depth < 1 {
		return nil
	}

	for i := 0; i < testRootWidth; i++ {
		var subDir = filepath.Join(path, fmt.Sprintf("folder%04d", i+1))

		if err = populateTestRoot(subDir, depth-1); err != nil {
			return err
		}
	}

	return nil
} // func populateTestRoot(path string, depth int) error

func randomFilename(idx int) string {
	var suffixList = []string{
		"txt",
		"go",
		"pl",
		"mp4",
		"opus",
		"jpg",
		"odt",
		"docx",
	}

	return fmt.Sprintf("%04d%016x.%s",
		idx,
		rand.Int63(),
		suffixList[rand.Intn(len(suffixList))])
} // func randomFilename() string
