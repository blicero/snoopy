// /home/krylon/go/src/github.com/blicero/snoopy/extractor/01_extractor_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 10. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-01-11 16:59:59 krylon>

package extractor

import (
	"testing"

	"github.com/blicero/snoopy/model"
)

var samples = map[string]probe{
	"./testdata/20250109_151326.jpg":        processImage,
	"./testdata/Athens_-_Pierce_Murphy.mp3": processAudio,
	"./testdata/bear_hugging_deer.jpg":      processImage,
	"./testdata/kant_rvernunft_1781.txt":    processPlaintext,
}

func TestExtractor(t *testing.T) {
	for path, ex := range samples {
		var (
			err  error
			meta *model.FileMeta
			f    = &model.File{
				RootID: 1,
				Path:   path,
			}
		)

		if meta, err = ex(f); err != nil {
			t.Errorf("Failed to extract metadata from %s: %s",
				path,
				err.Error())
			continue
		} else if meta == nil {
			t.Error("Extractor function did not return an error, but it did not return a valid FileMeta object")
		}

		// t.Logf("Metadata for %s:\n%#v\n\n",
		// 	path,
		// 	meta)
	}
} // func TestExtractor(t *testing.T)
