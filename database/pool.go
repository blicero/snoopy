// /home/krylon/go/src/github.com/blicero/snoopy/database/pool.go
// -*- mode: go; coding: utf-8; -*-
// Created on 07. 06. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-02-03 18:40:31 krylon>

package database

import (
	"fmt"
	"log"
	"sync"

	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/common/path"
	"github.com/blicero/snoopy/logdomain"
)

type dblink struct {
	db   *Database
	next *dblink
}

// Pool is a pool of database connections
type Pool struct {
	cnt   int
	log   *log.Logger
	link  *dblink
	lock  sync.RWMutex
	empty *sync.Cond
}

const poolSize = 8

var (
	thePool  *Pool
	initPool sync.Once
)

func makePool(cnt int) (*Pool, error) {
	var (
		err  error
		pool = &Pool{cnt: poolSize}
	)

	pool.empty = sync.NewCond(&pool.lock)

	if cnt < 1 {
		return nil, fmt.Errorf(
			"NewPool expects a positive number, you passed %d",
			cnt)
	} else if pool.log, err = common.GetLogger(logdomain.DBPool); err != nil {
		return nil, err
	}

	for i := 0; i < cnt; i++ {
		var link = &dblink{next: pool.link}

		if link.db, err = Open(common.Path(path.Database)); err != nil {
			pool.log.Printf("[ERROR] Cannot open database: %s\n",
				err.Error())
			return nil, err
		}

		pool.link = link
	}

	return pool, nil
} // func makePool(cnt int) (*Pool, error)

func makeGlobalPool() {
	var (
		err error
		p   *Pool
	)

	if p, err = makePool(poolSize); err != nil {
		panic(err)
	} else {
		thePool = p
	}
}

// Get returns one connection from the global Pool, blocking if necessary.
func Get() *Database {
	initPool.Do(makeGlobalPool)

	return thePool.Get()
} // func Get() *Database

func Put(db *Database) {
	thePool.Put(db)
}

// NewPool creates a Pool of database connections.
// The number of connections to use is given by the
// parameter cnt.
func NewPool(cnt int) (*Pool, error) {
	initPool.Do(makeGlobalPool)

	return thePool, nil
} // func NewPool(cnt int) (*Pool, error)

// Close closes all open database connections currently in the pool and empties
// the pool. Any connections retrieved from the pool that are in use at the
// time Close is called are unaffected.
func (pool *Pool) Close() error {
	pool.lock.Lock()

	for link := pool.link; link != nil; link = link.next {
		if link.db != nil {
			link.db.Close() // nolint: errcheck,gosec
			link.db = nil
		}
	}

	pool.link = nil
	pool.cnt = 0
	pool.lock.Unlock()
	return nil
} // func (pool *Pool) Close() error

// Get returns a DB connection from the pool.
// If the pool is empty, it waits for a connection to be returned.
func (pool *Pool) Get() *Database {
	var link *dblink

	pool.lock.Lock()
	defer pool.lock.Unlock()

WAIT_FOR_LINK:
	if pool.link != nil {
		link = pool.link
		pool.link = link.next
		pool.cnt--

		link.next = nil
		return link.db
	}

	// Wait for it!!!
	pool.empty.Wait()
	goto WAIT_FOR_LINK
} // func (pool *Pool) Get() *Database

// GetNoWait returns a DB connection from the pool.
// If the pool is empty, it creates a new one.
func (pool *Pool) GetNoWait() (*Database, error) {
	var db *Database
	var err error

	pool.lock.Lock()
	defer pool.lock.Unlock()

	if pool.link != nil {
		link := pool.link
		pool.link = link.next
		pool.cnt--
		return link.db, nil
	} else if db, err = Open(common.Path(path.Database)); err != nil {
		pool.log.Printf("[ERROR] Error opening new database connection: %s",
			err.Error())
		return nil, err
	}

	return db, nil
} // func (pool *Pool) GetNoWait() *Database

// Put returns a DB connection to the pool.
func (pool *Pool) Put(db *Database) {
	link := &dblink{
		db: db,
	}

	if db.tx != nil {
		pool.log.Println("[INFO] DB has pending transaction, rolling back.")
		if err := db.Rollback(); err != nil {
			pool.log.Printf("[ERROR] Cannot roll back transaction: %s\n",
				err.Error())
		}
	}

	pool.lock.Lock()
	link.next = pool.link
	pool.link = link
	pool.cnt++
	pool.lock.Unlock()
	pool.empty.Signal()
} // func (pool *Pool) Put(db *Database)

// IsEmpty returns true if the pool is currently empty.
func (pool *Pool) IsEmpty() bool {
	pool.lock.RLock()
	var empty = pool.link == nil
	pool.lock.RUnlock()
	return empty
} // func (pool *Pool) IsEmpty() bool
