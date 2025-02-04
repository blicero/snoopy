// /home/krylon/go/src/github.com/blicero/snoopy/extractor/extractor.go
// -*- mode: go; coding: utf-8; -*-
// Created on 08. 01. 2025 by Benjamin Walkenhorst
// (c) 2025 Benjamin Walkenhorst
// Time-stamp: <2025-02-04 20:09:00 krylon>

// Package extractor deals with extracting (hence the name - duh!) searchable
// metadata from the files the Walker has found.
package extractor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	"github.com/dhowden/tag"
	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/exif2"
)

const (
	bigFile = 64 * 1024 * 1024 // 64 MiB
	timeout = time.Second * 90
)

// ErrTooLarge indicates that a file is too large to be processed.
// ErrNoProbe indicates we have no probe to get metadata from a file type.
var (
	ErrTooLarge = errors.New("File is too large")
	ErrNoProbe  = errors.New("No matching probe was found for the given type")
)

var (
	tesseract string
	pdf2txt   string
	odt2txt   string
	docx2txt  string
)

func init() {
	var err error

	if tesseract, err = exec.LookPath("tesseract"); err != nil {
		tesseract = ""
		fmt.Fprintf(
			os.Stderr,
			"Failed to look up tesseract: %s\n",
			err.Error())
	} else {
		fmt.Printf("Using tesseract at %s\n", tesseract)
	}

	if pdf2txt, err = exec.LookPath("pdf2txt"); err != nil {
		pdf2txt = ""
		fmt.Fprintf(
			os.Stderr,
			"Failed to look up pdf2txt: %s\n",
			err.Error())
	} else {
		fmt.Printf("Using pdf2txt at %s\n", pdf2txt)
	}

	if odt2txt, err = exec.LookPath("odt2txt"); err != nil {
		odt2txt = ""
		fmt.Fprintf(
			os.Stderr,
			"Failed to look up odt2txt: %s\n",
			err.Error())
	} else {
		fmt.Printf("Using odt2txt at %s\n", odt2txt)
	}

	if docx2txt, err = exec.LookPath("docx2txt"); err != nil {
		docx2txt = ""
		fmt.Fprintf(
			os.Stderr,
			"Failed to look up docx2txt: %s\n",
			err.Error())
	} else {
		fmt.Printf("Using docx2txt at %s\n", docx2txt)
	}
} // func init()

type probe func(*model.File) (*model.FileMeta, error)

var workers = map[string]probe{
	"application/pdf":                         processPdf,
	"application/vnd.oasis.opendocument.text": processOdt,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": processDocx,
	"audio/mp4":                 processAudio,
	"audio/mpeg":                processAudio,
	"audio/ogg":                 processAudio,
	"image/bmp":                 processImage,
	"image/gif":                 processImage,
	"image/jpeg":                processImage,
	"image/png":                 processImage,
	"image/tiff":                processImage,
	"image/webp":                processImage,
	"text/plain":                processPlaintext,
	"text/plain; charset=utf-8": processPlaintext,
}

// Extractor wraps the handling of various file types to extract searchable
// metadata / content from.
type Extractor struct {
	log       *log.Logger
	pool      *database.Pool
	handlers  map[string]probe
	active    atomic.Bool
	workerCnt int
}

// New creates a new Extractor.
func New() (*Extractor, error) {
	var (
		err error
		ex  = &Extractor{
			handlers:  maps.Clone(workers),
			workerCnt: max(runtime.NumCPU()/2, 1),
		}
	)

	if ex.log, err = common.GetLogger(logdomain.Extractor); err != nil {
		return nil, err
	} else if ex.pool, err = database.NewPool(runtime.NumCPU()); err != nil {
		ex.log.Printf("[ERROR] Failed to create Database Pool: %s\n",
			err.Error())
		return nil, err
	}

	return ex, nil
} // func New() (*Extractor, error)

// IsActive returns the Extractor's active flag
func (ex *Extractor) IsActive() bool {
	return ex.active.Load()
} // func (ex *Extractor) IsActive() bool

// Stop clears the Extractor's active flag
func (ex *Extractor) Stop() {
	ex.active.Store(false)
} // func (ex *Extractor) Stop()

// Run attempts to process all Files in the database that do not have up-to-date
// Metadata.
func (ex *Extractor) Run() {
	ex.active.Store(true)
	defer ex.active.Store(false)

	var (
		err          error
		db           *database.Database
		files        []*model.File
		outdatedMeta []*model.FileMeta
		q            chan *model.File
		wg           sync.WaitGroup
	)

	q = make(chan *model.File)
	wg.Add(ex.workerCnt)

	for i := 0; i < ex.workerCnt; i++ {
		go ex.worker(q, &wg)
	}

	db = ex.pool.Get()
	defer ex.pool.Put(db)

	if files, err = db.FileGetNoMeta(); err != nil {
		ex.log.Printf("[ERROR] Failed to get Files without Metadata: %s\n",
			err.Error())
		close(q)
		return
	} else if outdatedMeta, err = db.MetaGetOutdated(); err != nil {
		ex.log.Printf("[ERROR] Failed to load outdated Metadata: %s\n",
			err.Error())
		close(q)
		return
	}

	for _, f := range files {
		q <- f
	}

	for _, m := range outdatedMeta {
		q <- m.F
	}

	close(q)
	wg.Wait()

} // func (ex *Extractor) Run()

func (ex *Extractor) worker(q <-chan *model.File, wg *sync.WaitGroup) {
	defer wg.Done()

	var ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	for ex.IsActive() {
		var (
			f  *model.File
			ok bool
		)
		select {
		case <-ticker.C:
			continue
		case f, ok = <-q:
			if !ok {
				return
			}

			// Process file
			var (
				err error
				m   *model.FileMeta
			)

			if m, err = ex.Process(f); err != nil {
				if err != ErrNoProbe {
					ex.log.Printf("[ERROR] Failed to extract Metadata from File %s (%d): %s\n",
						f.Path,
						f.ID,
						err.Error())
				}
				continue
			} else if err = ex.saveMeta(m); err != nil {
				ex.log.Printf("[ERROR] Failed to save Metadata for File %s (%d): %s\n",
					f.Path,
					f.ID,
					err.Error())
			}
		}
	}
} // func (ex *Extractor) worker(q <- chan *model.File, wg *sync.WaitGroup)

func (ex *Extractor) saveMeta(m *model.FileMeta) error {
	var (
		err error
		db  *database.Database
	)

	db = ex.pool.Get()
	defer ex.pool.Put(db)

	if err = db.MetaUpsert(m); err != nil {
		ex.log.Printf("[ERROR] Failed UPSERT Metadata for File %s (%d): %s\n",
			m.F.Path,
			m.FileID,
			err.Error())
		return err
	}

	return nil
} // func (ex *Extractor) saveMeta(m *model.FileMeta) error

// Process attempts to extract usable information from a file to use in a
// search index.
func (ex *Extractor) Process(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		meta *model.FileMeta
		w    probe
		ok   bool
	)

	if w, ok = ex.handlers[f.Type]; !ok {
		// ex.log.Printf("[INFO] No probe for %s (%s) was found.\n",
		// 	f.Path,
		// 	f.Type)
		return nil, ErrNoProbe
	} else if meta, err = w(f); err != nil {
		ex.log.Printf("[ERROR] Failed to extract metadata from %s: %s\n",
			f.Path,
			err.Error())
		return nil, err
	}

	meta.F = f

	return meta, nil
} // func (ex *Extractor) Process(f *model.File) (*model.FileMeta, error)

func processPdf(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		meta = &model.FileMeta{
			FileID:    f.ID,
			F:         f,
			Timestamp: time.Now(),
			Meta:      make(map[string]string),
		}
	)

	if meta.Content, err = runPdf2Txt(f); err != nil {
		return nil, err
	}

	return meta, nil
} // func processPdf(f *model.File) (*model.FileMeta, error)

func processOdt(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		meta = &model.FileMeta{
			FileID:    f.ID,
			F:         f,
			Timestamp: time.Now(),
			Meta:      make(map[string]string),
		}
	)

	if meta.Content, err = runOdt2Txt(f); err != nil {
		return nil, err
	}

	return meta, nil
} // func processOdt(f *model.File) (*model.FileMeta, error)

func processDocx(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		meta = &model.FileMeta{
			FileID:    f.ID,
			F:         f,
			Timestamp: time.Now(),
			Meta:      make(map[string]string),
		}
	)

	if meta.Content, err = runDocx2Txt(f); err != nil {
		return nil, err
	}

	return meta, nil
} // func processDocx(f *model.File) (*model.FileMeta, error)

func processPlaintext(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		raw  []byte
		meta *model.FileMeta
		info os.FileInfo
	)

	if info, err = os.Stat(f.Path); err != nil {
		return nil, err
	} else if info.Size() > bigFile {
		return nil, ErrTooLarge
	} else if raw, err = os.ReadFile(f.Path); err != nil {
		return nil, err
	}

	meta = &model.FileMeta{
		FileID:    f.ID,
		Timestamp: time.Now(),
		Content:   string(raw),
		Meta:      make(map[string]string),
	}

	return meta, nil
} // func processPlaintext(f *model.File) (*model.FileMeta, error)

func processAudio(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		fh   *os.File
		meta *model.FileMeta
		am   tag.Metadata
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if am, err = tag.ReadFrom(fh); err != nil {
		return nil, err
	}

	meta = &model.FileMeta{
		FileID:    f.ID,
		Timestamp: time.Now(),
	}

	meta.Meta = map[string]string{
		"Title":  am.Title(),
		"Artist": am.Artist(),
		"Album":  am.Album(),
		"Year":   strconv.FormatInt(int64(am.Year()), 10),
	}

	return meta, nil
} // func processAudio(f *model.File) (*model.FileMeta, error)

func processImage(f *model.File) (*model.FileMeta, error) {
	var (
		err  error
		fh   *os.File
		im   exif2.Exif
		meta = &model.FileMeta{
			FileID:    f.ID,
			Timestamp: time.Now(),
		}
	)

	if fh, err = os.Open(f.Path); err != nil {
		return nil, err
	}

	defer fh.Close() // nolint: errcheck

	if im, err = imagemeta.Decode(fh); err == nil {
		meta.Meta = map[string]string{
			"Date":        im.GPS.Date().Format(common.TimestampFormat),
			"Latitude":    strconv.FormatFloat(im.GPS.Latitude(), 'f', -1, 32),
			"Longitude":   strconv.FormatFloat(im.GPS.Longitude(), 'f', -1, 32),
			"XResolution": strconv.FormatUint(uint64(im.XResolution), 10),
			"YResolution": strconv.FormatUint(uint64(im.YResolution), 10),
		}
	}

	if meta.Content, err = runTesseract(f); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Failed to extract text using TesseracT OCR engine from %s: %s\n",
			f.Path,
			err.Error())
		meta.Content = ""
	}

	return meta, nil
} // func processImage(f *model.File) (*model.FileMeta, error)

func runTesseract(f *model.File) (string, error) {
	// TODO Pipe the output of tesseract(1) instead of using a temporary file?
	//	If not, at least remove the output file
	var (
		err     error
		cmd     *exec.Cmd
		fh      *os.File
		bld     strings.Builder
		outfile string
		rnd     uint32
	)

	if tesseract == "" {
		return "", nil
	}

	rnd = rand.Uint32()
	outfile = filepath.Join(
		os.TempDir(),
		fmt.Sprintf("snoopy_tesseract_%08x", rnd),
	)

	cmd = exec.Command("nice", tesseract, f.Path, outfile)

	if err = cmd.Run(); err != nil {
		return "", err
	}

	outfile = outfile + ".txt"
	defer os.Remove(outfile) // nolint: errcheck

	if fh, err = os.Open(outfile); err != nil {
		return "", err
	}

	defer fh.Close()

	if _, err = io.Copy(&bld, fh); err != nil {
		return "", err
	}

	return bld.String(), nil
} // func runTesseract(f *model.File) (string, error)

func runPdf2Txt(f *model.File) (string, error) {
	var (
		err            error
		cmd            *exec.Cmd
		ctx            context.Context
		cancel         context.CancelFunc
		stdout, stderr strings.Builder
	)

	/* Gefunden unter https://stackoverflow.com/questions/41507805/optional-timeouts-in-golang

	ctx := context.Background()
	    if timeout > 0 {
	        var cancel context.CancelFunc
	        ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	        defer cancel()
	    }

	    cmd := exec.CommandContext(ctx, "sleep", "5")

	    if err := cmd.Run(); err != nil {
	        panic(err)
	    }
	*/

	if pdf2txt == "" {
		return "", nil
	}

	ctx = context.Background()
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, "nice", pdf2txt, f.Path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error running pdf2txt: %s\n",
			stderr.String())
		return "", err
	}

	return stdout.String(), nil
} // func runPdf2Txt(f *model.File) (string, error)

func runOdt2Txt(f *model.File) (string, error) {
	var (
		err            error
		cmd            *exec.Cmd
		ctx            context.Context
		cancel         context.CancelFunc
		stdout, stderr strings.Builder
	)

	/* Gefunden unter https://stackoverflow.com/questions/41507805/optional-timeouts-in-golang

	ctx := context.Background()
	    if timeout > 0 {
	        var cancel context.CancelFunc
	        ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	        defer cancel()
	    }

	    cmd := exec.CommandContext(ctx, "sleep", "5")

	    if err := cmd.Run(); err != nil {
	        panic(err)
	    }
	*/

	if odt2txt == "" {
		return "", nil
	}

	ctx = context.Background()
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, "nice", odt2txt, f.Path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error running odt2txt: %s\n",
			stderr.String())
		return "", err
	}

	return stdout.String(), nil
} // func runOdt2Txt(f *model.File) (string, error)

func runDocx2Txt(f *model.File) (string, error) {
	var (
		err            error
		cmd            *exec.Cmd
		ctx            context.Context
		cancel         context.CancelFunc
		stdout, stderr strings.Builder
	)

	/* Gefunden unter https://stackoverflow.com/questions/41507805/optional-timeouts-in-golang

	ctx := context.Background()
	    if timeout > 0 {
	        var cancel context.CancelFunc
	        ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	        defer cancel()
	    }

	    cmd := exec.CommandContext(ctx, "sleep", "5")

	    if err := cmd.Run(); err != nil {
	        panic(err)
	    }
	*/

	if docx2txt == "" {
		return "", nil
	}

	ctx = context.Background()
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, "nice", docx2txt, f.Path)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error running odt2txt: %s\n",
			stderr.String())
		return "", err
	}

	return stdout.String(), nil
} // func runDocx2Txt(f *model.File) (string, error)
