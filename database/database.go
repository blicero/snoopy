// /home/krylon/go/src/github.com/blicero/snoopy/database/database.go
// -*- mode: go; coding: utf-8; -*-
// Created on 23. 12. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2025-01-11 18:11:53 krylon>

// Package database provides the persistence layer for the application.
package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/blicero/krylib"
	"github.com/blicero/snoopy/blacklist"
	"github.com/blicero/snoopy/common"
	"github.com/blicero/snoopy/database/query"
	"github.com/blicero/snoopy/logdomain"
	"github.com/blicero/snoopy/model"
	_ "github.com/mattn/go-sqlite3" // Import the database driver
)

var (
	openLock sync.Mutex
	idCnt    int64
)

// ErrTxInProgress indicates that an attempt to initiate a transaction failed
// because there is already one in progress.
var ErrTxInProgress = errors.New("A Transaction is already in progress")

// ErrNoTxInProgress indicates that an attempt was made to finish a
// transaction when none was active.
var ErrNoTxInProgress = errors.New("There is no transaction in progress")

// ErrEmptyUpdate indicates that an update operation would not change any
// values.
var ErrEmptyUpdate = errors.New("Update operation does not change any values")

// ErrInvalidValue indicates that one or more parameters passed to a method
// had values that are invalid for that operation.
var ErrInvalidValue = errors.New("Invalid value for parameter")

// ErrObjectNotFound indicates that an Object was not found in the database.
var ErrObjectNotFound = errors.New("object was not found in database")

// ErrInvalidSavepoint is returned when a user of the Database uses an unkown
// (or expired) savepoint name.
var ErrInvalidSavepoint = errors.New("that save point does not exist")

// If a query returns an error and the error text is matched by this regex, we
// consider the error as transient and try again after a short delay.
var retryPat = regexp.MustCompile("(?i)database is (?:locked|busy)")

// worthARetry returns true if an error returned from the database
// is matched by the retryPat regex.
func worthARetry(e error) bool {
	return retryPat.MatchString(e.Error())
} // func worthARetry(e error) bool

// retryDelay is the amount of time we wait before we repeat a database
// operation that failed due to a transient error.
const retryDelay = 25 * time.Millisecond

func waitForRetry() {
	time.Sleep(retryDelay)
} // func waitForRetry()

// Database wraps a database connection and associated state.
type Database struct {
	id            int64
	db            *sql.DB
	tx            *sql.Tx
	log           *log.Logger
	path          string
	spNameCounter int
	spNameCache   map[string]string
	queries       map[query.ID]*sql.Stmt
}

// Open opens a Database. If the database specified by the path does not exist,
// yet, it is created and initialized.
func Open(path string) (*Database, error) {
	var (
		err      error
		dbExists bool
		db       = &Database{
			path:          path,
			spNameCounter: 1,
			spNameCache:   make(map[string]string),
			queries:       make(map[query.ID]*sql.Stmt),
		}
	)

	openLock.Lock()
	defer openLock.Unlock()
	idCnt++
	db.id = idCnt

	if db.log, err = common.GetLogger(logdomain.Database); err != nil {
		return nil, err
	} else if common.Debug {
		db.log.Printf("[DEBUG] Open database %s\n", path)
	}

	var connstring = fmt.Sprintf("%s?_locking=NORMAL&_journal=WAL&_fk=true&recursive_triggers=true",
		path)

	if dbExists, err = krylib.Fexists(path); err != nil {
		db.log.Printf("[ERROR] Failed to check if %s already exists: %s\n",
			path,
			err.Error())
		return nil, err
	} else if db.db, err = sql.Open("sqlite3", connstring); err != nil {
		db.log.Printf("[ERROR] Failed to open %s: %s\n",
			path,
			err.Error())
		return nil, err
	}

	if !dbExists {
		if err = db.initialize(); err != nil {
			var e2 error
			if e2 = db.db.Close(); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to close database: %s\n",
					e2.Error())
				return nil, e2
			} else if e2 = os.Remove(path); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to remove database file %s: %s\n",
					db.path,
					e2.Error())
			}
			return nil, err
		}
		db.log.Printf("[INFO] Database at %s has been initialized\n",
			path)
	}

	return db, nil
} // func Open(path string) (*Database, error)

func (db *Database) initialize() error {
	var err error
	var tx *sql.Tx

	if common.Debug {
		db.log.Printf("[DEBUG] Initialize fresh database at %s\n",
			db.path)
	}

	if tx, err = db.db.Begin(); err != nil {
		db.log.Printf("[ERROR] Cannot begin transaction: %s\n",
			err.Error())
		return err
	}

	for _, q := range initQueries {
		db.log.Printf("[TRACE] Execute init query:\n%s\n",
			q)
		if _, err = tx.Exec(q); err != nil {
			db.log.Printf("[ERROR] Cannot execute init query: %s\n%s\n",
				err.Error(),
				q)
			if rbErr := tx.Rollback(); rbErr != nil {
				db.log.Printf("[CANTHAPPEN] Cannot rollback transaction: %s\n",
					rbErr.Error())
				return rbErr
			}
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		db.log.Printf("[CANTHAPPEN] Failed to commit init transaction: %s\n",
			err.Error())
		return err
	}

	return nil
} // func (db *Database) initialize() error

// Close closes the database.
// If there is a pending transaction, it is rolled back.
func (db *Database) Close() error {
	// I wonder if would make more snese to panic() if something goes wrong

	var err error

	if db.tx != nil {
		if err = db.tx.Rollback(); err != nil {
			db.log.Printf("[CRITICAL] Cannot roll back pending transaction: %s\n",
				err.Error())
			return err
		}
		db.tx = nil
	}

	for key, stmt := range db.queries {
		if err = stmt.Close(); err != nil {
			db.log.Printf("[CRITICAL] Cannot close statement handle %s: %s\n",
				key,
				err.Error())
			return err
		}
		delete(db.queries, key)
	}

	if err = db.db.Close(); err != nil {
		db.log.Printf("[CRITICAL] Cannot close database: %s\n",
			err.Error())
	}

	db.db = nil
	return nil
} // func (db *Database) Close() error

func (db *Database) getQuery(id query.ID) (*sql.Stmt, error) {
	var (
		stmt  *sql.Stmt
		found bool
		err   error
	)

	if stmt, found = db.queries[id]; found {
		return stmt, nil
	} else if _, found = dbQueries[id]; !found {
		return nil, fmt.Errorf("Unknown Query %d",
			id)
	}

	db.log.Printf("[TRACE] Prepare query %s\n", id)

PREPARE_QUERY:
	if stmt, err = db.db.Prepare(dbQueries[id]); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto PREPARE_QUERY
		}

		db.log.Printf("[ERROR] Cannot parse query %s: %s\n%s\n",
			id,
			err.Error(),
			dbQueries[id])
		return nil, err
	}

	db.queries[id] = stmt
	return stmt, nil
} // func (db *Database) getQuery(query.ID) (*sql.Stmt, error)

func (db *Database) resetSPNamespace() {
	db.spNameCounter = 1
	db.spNameCache = make(map[string]string)
} // func (db *Database) resetSPNamespace()

func (db *Database) generateSPName(name string) string {
	var spname = fmt.Sprintf("Savepoint%05d",
		db.spNameCounter)

	db.spNameCache[name] = spname
	db.spNameCounter++
	return spname
} // func (db *Database) generateSPName() string

// PerformMaintenance performs some maintenance operations on the database.
// It cannot be called while a transaction is in progress and will block
// pretty much all access to the database while it is running.
func (db *Database) PerformMaintenance() error {
	var mQueries = []string{
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"VACUUM",
		"REINDEX",
		"ANALYZE",
	}
	var err error

	if db.tx != nil {
		return ErrTxInProgress
	}

	for _, q := range mQueries {
		if _, err = db.db.Exec(q); err != nil {
			db.log.Printf("[ERROR] Failed to execute %s: %s\n",
				q,
				err.Error())
		}
	}

	return nil
} // func (db *Database) PerformMaintenance() error

// Begin begins an explicit database transaction.
// Only one transaction can be in progress at once, attempting to start one,
// while another transaction is already in progress will yield ErrTxInProgress.
func (db *Database) Begin() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Begin Transaction\n",
		db.id)

	if db.tx != nil {
		return ErrTxInProgress
	}

BEGIN_TX:
	for db.tx == nil {
		if db.tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				continue BEGIN_TX
			} else {
				db.log.Printf("[ERROR] Failed to start transaction: %s\n",
					err.Error())
				return err
			}
		}
	}

	db.resetSPNamespace()

	return nil
} // func (db *Database) Begin() error

// SavepointCreate creates a savepoint with the given name.
//
// Savepoints only make sense within a running transaction, and just like
// with explicit transactions, managing them is the responsibility of the
// user of the Database.
//
// Creating a savepoint without a surrounding transaction is not allowed,
// even though SQLite allows it.
//
// For details on how Savepoints work, check the excellent SQLite
// documentation, but here's a quick guide:
//
// Savepoints are kind-of-like transactions within a transaction: One
// can create a savepoint, make some changes to the database, and roll
// back to that savepoint, discarding all changes made between
// creating the savepoint and rolling back to it. Savepoints can be
// quite useful, but there are a few things to keep in mind:
//
//   - Savepoints exist within a transaction. When the surrounding transaction
//     is finished, all savepoints created within that transaction cease to exist,
//     no matter if the transaction is commited or rolled back.
//
//   - When the database is recovered after being interrupted during a
//     transaction, e.g. by a power outage, the entire transaction is rolled back,
//     including all savepoints that might exist.
//
//   - When a savepoint is released, nothing changes in the state of the
//     surrounding transaction. That means rolling back the surrounding
//     transaction rolls back the entire transaction, regardless of any
//     savepoints within.
//
//   - Savepoints do not nest. Releasing a savepoint releases it and *all*
//     existing savepoints that have been created before it. Rolling back to a
//     savepoint removes that savepoint and all savepoints created after it.
func (db *Database) SavepointCreate(name string) error {
	var err error

	db.log.Printf("[DEBUG] SavepointCreate(%s)\n",
		name)

	if db.tx == nil {
		return ErrNoTxInProgress
	}

SAVEPOINT:
	// It appears that the SAVEPOINT statement does not support placeholders.
	// But I do want to used named savepoints.
	// And I do want to use the given name so that no SQL injection
	// becomes possible.
	// It would be nice if the database package or at least the SQLite
	// driver offered a way to escape the string properly.
	// One possible solution would be to use names generated by the
	// Database instead of user-defined names.
	//
	// But then I need a way to use the Database-generated name
	// in rolling back and releasing the savepoint.
	// I *could* use the names strictly inside the Database, store them in
	// a map or something and hand out a key to that name to the user.
	// Since savepoint only exist within one transaction, I could even
	// re-use names from one transaction to the next.
	//
	// Ha! I could accept arbitrary names from the user, generate a
	// clean name, and store these in a map. That way the user can
	// still choose names that are outwardly visible, but they do
	// not touch the Database itself.
	//
	//if _, err = db.tx.Exec("SAVEPOINT ?", name); err != nil {
	// if _, err = db.tx.Exec("SAVEPOINT " + name); err != nil {
	// 	if worthARetry(err) {
	// 		waitForRetry()
	// 		goto SAVEPOINT
	// 	}

	// 	db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
	// 		name,
	// 		err.Error())
	// }

	var internalName = db.generateSPName(name)

	var spQuery = "SAVEPOINT " + internalName

	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	return err
} // func (db *Database) SavepointCreate(name string) error

// SavepointRelease releases the Savepoint with the given name, and all
// Savepoints created before the one being release.
func (db *Database) SavepointRelease(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRelease(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		db.log.Printf("[ERROR] Attempt to release unknown Savepoint %q\n",
			name)
		return ErrInvalidSavepoint
	}

	db.log.Printf("[DEBUG] Release Savepoint %q (%q)",
		name,
		db.spNameCache[name])

	spQuery = "RELEASE SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to release savepoint %s: %s\n",
			name,
			err.Error())
	} else {
		delete(db.spNameCache, internalName)
	}

	return err
} // func (db *Database) SavepointRelease(name string) error

// SavepointRollback rolls back the running transaction to the given savepoint.
func (db *Database) SavepointRollback(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRollback(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		return ErrInvalidSavepoint
	}

	spQuery = "ROLLBACK TO SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	delete(db.spNameCache, name)
	return err
} // func (db *Database) SavepointRollback(name string) error

// Rollback terminates a pending transaction, undoing any changes to the
// database made during that transaction.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Rollback() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Roll back Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Rollback(); err != nil {
		return fmt.Errorf("Cannot roll back database transaction: %s",
			err.Error())
	}

	db.tx = nil
	db.resetSPNamespace()

	return nil
} // func (db *Database) Rollback() error

// Commit ends the active transaction, making any changes made during that
// transaction permanent and visible to other connections.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Commit() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Commit Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Commit(); err != nil {
		return fmt.Errorf("Cannot commit transaction: %s",
			err.Error())
	}

	db.resetSPNamespace()
	db.tx = nil
	return nil
} // func (db *Database) Commit() error

// RootAdd adds a Root folder to the database
func (db *Database) RootAdd(r *model.Root) error {
	const qid query.ID = query.RootAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(r.Path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add root %s to database: %s",
				r.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added root %s: %s",
				r.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		r.ID = id
		status = true
		return nil
	}
} // func (db *Database) RootAdd(r *model.Root) error

// RootGetByPath loads a Root directory by its path
func (db *Database) RootGetByPath(path string) (*model.Root, error) {
	const qid query.ID = query.RootGetByPath
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			r         = &model.Root{Path: path}
		)

		if err = rows.Scan(&r.ID, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for Root %s: %s",
				path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		r.LastScan = time.Unix(timestamp, 0)

		return r, nil
	}

	db.log.Printf("[INFO] Root %s was not found in database\n", path)
	return nil, nil
} // func (db *Database) RootGetByPath(path string) (*model.Root, error)

// RootGetByID loads a Root directory by its ID
func (db *Database) RootGetByID(id int64) (*model.Root, error) {
	const qid query.ID = query.RootGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			r         = &model.Root{ID: id}
		)

		if err = rows.Scan(&r.Path, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for Root %d: %s",
				id,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		r.LastScan = time.Unix(timestamp, 0)

		return r, nil
	}

	db.log.Printf("[INFO] Root %d was not found in database\n", id)
	return nil, nil
} // func (db *Database) RootGetByID(id int64) (*model.Root, error)

// RootGetAll fetches all Roots from the database.
func (db *Database) RootGetAll() ([]*model.Root, error) {
	const qid query.ID = query.RootGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var roots = make([]*model.Root, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			r         = new(model.Root)
		)

		if err = rows.Scan(&r.ID, &r.Path, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for Root: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		r.LastScan = time.Unix(timestamp, 0)

		roots = append(roots, r)
	}

	return roots, nil
} // func (db *Database) RootGetAll() ([]*model.Root, error)

// RootDelete deletes a Root directory from the database.
func (db *Database) RootDelete(r *model.Root) error {
	const qid query.ID = query.RootDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(r.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add root %s to database: %s",
				r.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) RootDelete(r *model.Root) error

// RootMarkScan updates a Root's LastScan timestamp
func (db *Database) RootMarkScan(r *model.Root, timestamp time.Time) error {
	const qid query.ID = query.RootMarkScan
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(timestamp.Unix(), r.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add root %s to database: %s",
				r.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	r.LastScan = timestamp
	status = true
	return nil
} // func (db *Database) RootMarkScan(r *model.Root, timestamp time.Time) error

// FileAdd adds a File to the database
func (db *Database) FileAdd(f *model.File) error {
	const qid query.ID = query.FileAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(f.RootID, f.Path, f.Type, f.CTime.Unix()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add File %s to database: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added root %s: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		f.ID = id
		status = true
		return nil
	}
} // func (db *Database) FileAdd(f *model.File) error

// FileDelete removes a File from the database
func (db *Database) FileDelete(f *model.File) error {
	const qid query.ID = query.FileDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add root %s to database: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) FileDelete(f *model.File) error

// FileUpdateCtime updates a File's CTime
func (db *Database) FileUpdateCtime(f *model.File, ctime time.Time) error {
	const qid query.ID = query.FileUpdateCtime
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(ctime.Unix(), f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update CTime of File %s: %s",
				f.Path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	f.CTime = ctime
	status = true
	return nil
} // func (db *Database) FileUpdateCtime(f *model.File, ctime time.Time) error

// FileGetByPath fetches a File by its path
func (db *Database) FileGetByPath(path string) (*model.File, error) {
	const qid query.ID = query.FileGetByPath
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(path); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			f         = &model.File{Path: path}
		)

		if err = rows.Scan(&f.ID, &f.RootID, &f.Type, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for Root %s: %s",
				path,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(timestamp, 0)

		return f, nil
	}

	db.log.Printf("[DEBUG] File %s was not found in database\n", path)
	return nil, nil
} // func (db *Database) FileGetByPath(path string) (*model.File, error)

// FileGetByID loads a File by its ID
func (db *Database) FileGetByID(id int64) (*model.File, error) {
	const qid query.ID = query.FileGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			f         = &model.File{ID: id}
		)

		if err = rows.Scan(&f.RootID, &f.Path, &f.Type, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for File %d: %s",
				id,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(timestamp, 0)

		return f, nil
	}

	db.log.Printf("[INFO] File %d was not found in database\n", id)
	return nil, nil
} // func (db *Database) FileGetByID(id int64) (*model.File, error)

// FileGetByPattern loads all Files whose Path matches the given pattern
func (db *Database) FileGetByPattern(pat string) ([]*model.File, error) {
	const qid query.ID = query.FileGetByPattern
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(pat); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var files = make([]*model.File, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			f         = new(model.File)
		)

		if err = rows.Scan(&f.ID, &f.RootID, &f.Path, &f.Type, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for File: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(timestamp, 0)
		files = append(files, f)
	}

	return files, nil
} // func (db *Database) FileGetByPattern(pat string) ([]*model.File, error)

// FileGetAll loads *all* Files from the database. Use with caution
func (db *Database) FileGetAll() ([]*model.File, error) {
	const qid query.ID = query.FileGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var files = make([]*model.File, 0, 64)

	for rows.Next() {
		var (
			timestamp int64
			f         = new(model.File)
		)

		if err = rows.Scan(&f.ID, &f.RootID, &f.Path, &f.Type, &timestamp); err != nil {
			msg = fmt.Sprintf("Error scanning row for File: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(timestamp, 0)
		files = append(files, f)
	}

	return files, nil
} // func (db *Database) FileGetAll(pat string) ([]*model.File, error)

// BlacklistAdd adds a new Blacklist Item to the database
func (db *Database) BlacklistAdd(item blacklist.Item) error {
	const qid query.ID = query.BlacklistAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var (
		rows    *sql.Rows
		pattern string
		isGlob  bool
	)

	db.log.Printf("[DEBUG] Add Blacklist Item %s\n", item.GetPattern())

	switch bitem := item.(type) {
	case *blacklist.ReItem:
		pattern = bitem.Pattern.String()
	case *blacklist.GlobItem:
		pattern = bitem.Raw
		isGlob = true
	}

EXEC_QUERY:
	if rows, err = stmt.Query(pattern, isGlob); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add BlacklistItem %s to database: %s",
				pattern,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added BlacklistItem %s: %s",
				item.GetPattern(),
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		switch bitem := item.(type) {
		case *blacklist.ReItem:
			bitem.ID = id
		case *blacklist.GlobItem:
			bitem.ID = id
		}

		status = true
		return nil
	}
} // func (db *Database) BlacklistAdd(item blacklist.Item) error

// BlacklistHit increases the hit count for the given Item
func (db *Database) BlacklistHit(item blacklist.Item) error {
	const qid query.ID = query.BlacklistHit
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(item.GetID()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update hit count of Blacklist Item %d: %s",
				item.GetID(),
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) BlacklistHit(item blacklist.Item) error

// BlacklistGetAll loads all Blacklist Items from the database
func (db *Database) BlacklistGetAll() ([]blacklist.Item, error) {
	const qid query.ID = query.BlacklistGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]blacklist.Item, 0, 64)

	for rows.Next() {
		var (
			id, cnt int64
			pstring string
			isGlob  bool
			item    blacklist.Item
		)

		if err = rows.Scan(&id, &pstring, &isGlob, &cnt); err != nil {
			msg = fmt.Sprintf("Error scanning row for File: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if isGlob {
			if item, err = blacklist.NewGlobItem(id, cnt, pstring); err != nil {
				db.log.Printf("[ERROR] Failed to create GlobItem: %s\n",
					err.Error())
				return nil, err
			}
		} else {
			if item, err = blacklist.NewReItem(id, cnt, pstring); err != nil {
				db.log.Printf("[ERROR] Failed to create ReItem: %s\n",
					err.Error())
				return nil, err
			}
		}

		items = append(items, item)
	}

	return items, nil
} // func (db *Database) BlacklistGetAll() ([]blacklist.Item, error)

// MetaAdd adds metadata for a specific File to the database
func (db *Database) MetaAdd(m *model.FileMeta) error {
	const qid query.ID = query.MetaAdd
	var (
		err      error
		msg      string
		stmt     *sql.Stmt
		tx       *sql.Tx
		status   bool
		buf      []byte
		metaJSON string
	)

	if buf, err = json.Marshal(m.Meta); err != nil {
		db.log.Printf("[ERROR] Cannot convert Metadata to JSON: %s\n",
			err.Error())
	}

	metaJSON = string(buf)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(m.FileID, m.Timestamp.Unix(), m.Content, metaJSON); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Metadata for file %d to database: %s",
				m.FileID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added Metadata for File %d: %s",
				m.FileID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		m.ID = id
		status = true
		return nil
	}
} // func (db *Database) MetaAdd(m *model.FileMeta) error

// MetaGetByFile fetches the metadata of the given File, if it exists.
func (db *Database) MetaGetByFile(f *model.File) (*model.FileMeta, error) {
	const qid query.ID = query.MetaGetByFile
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			jMeta     string
			m         = &model.FileMeta{FileID: f.ID}
		)

		if err = rows.Scan(&m.ID, &timestamp, &m.Content, &jMeta); err != nil {
			msg = fmt.Sprintf("Error scanning row for FileMeta of File %d: %s",
				f.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		m.Timestamp = time.Unix(timestamp, 0)

		if err = json.Unmarshal([]byte(jMeta), &m.Meta); err != nil {
			db.log.Printf("[ERROR] Failed to parse JSON of Metadata for File %d: %s\n",
				f.ID,
				err.Error())
			return nil, err
		}

		return m, nil
	}

	db.log.Printf("[TRACE] Metadata of File %d was not found in database\n", f.ID)
	return nil, nil
} // func (db *Database) MetaGetByFile(f *model.File) (*model.FileMeta, error)

// MetaGetByRoot loads the metadata of all Files that belong to the given Root.
func (db *Database) MetaGetByRoot(r *model.Root) ([]*model.FileMeta, error) {
	const qid query.ID = query.MetaGetByRoot
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(r.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var results = make([]*model.FileMeta, 0, 64)

	for rows.Next() {
		var (
			timestamp, ctime int64
			jMeta            string
			f                = &model.File{RootID: r.ID}
			m                = new(model.FileMeta)
		)

		if err = rows.Scan(&m.ID, &m.FileID, &m.Content, &jMeta, &f.Path, &f.Type, &ctime); err != nil {
			msg = fmt.Sprintf("Error scanning row for Metadata: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(ctime, 0)
		m.Timestamp = time.Unix(timestamp, 0)
		f.ID = m.FileID
		m.F = f

		if err = json.Unmarshal([]byte(jMeta), &m.Meta); err != nil {
			db.log.Printf("[ERROR] Failed to parse JSON of Metadata of File %d: %s\n",
				m.FileID,
				err.Error())
			return nil, err
		}

		results = append(results, m)
	}

	return results, nil
} // func (db *Database) MetaGetByRoot(r *model.Root) ([]*model.FileMeta, error)

// MetaGetOutdated loads the metadata for all Files that have metadata whose
// timestamp is older than the File's CTime.
func (db *Database) MetaGetOutdated() ([]*model.FileMeta, error) {
	const qid query.ID = query.MetaGetOutdated
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var results = make([]*model.FileMeta, 0, 64)

	for rows.Next() {
		var (
			timestamp, ctime int64
			jMeta            string
			f                = new(model.File)
			m                = new(model.FileMeta)
		)

		if err = rows.Scan(&m.ID, &m.FileID, &m.Content, &jMeta, &f.RootID, &f.Path, &f.Type, &ctime); err != nil {
			msg = fmt.Sprintf("Error scanning row for Metadata: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(ctime, 0)
		m.Timestamp = time.Unix(timestamp, 0)
		f.ID = m.FileID
		m.F = f

		if err = json.Unmarshal([]byte(jMeta), &m.Meta); err != nil {
			db.log.Printf("[ERROR] Failed to parse JSON of Metadata of File %d: %s\n",
				m.FileID,
				err.Error())
			return nil, err
		}

		results = append(results, m)
	}

	return results, nil
} // func (db *Database) MetaGetOutdated() ([]*model.FileMeta, error)

// MetaGetAll loads ALL metadata from the database
func (db *Database) MetaGetAll() ([]*model.FileMeta, error) {
	const qid query.ID = query.MetaGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var results = make([]*model.FileMeta, 0, 64)

	for rows.Next() {
		var (
			timestamp, ctime int64
			jMeta            string
			f                = new(model.File)
			m                = new(model.FileMeta)
		)

		if err = rows.Scan(&m.ID, &m.FileID, &m.Content, &jMeta, &f.RootID, &f.Path, &f.Type, &ctime); err != nil {
			msg = fmt.Sprintf("Error scanning row for Metadata: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		f.CTime = time.Unix(ctime, 0)
		m.Timestamp = time.Unix(timestamp, 0)
		f.ID = m.FileID
		m.F = f

		if err = json.Unmarshal([]byte(jMeta), &m.Meta); err != nil {
			db.log.Printf("[ERROR] Failed to parse JSON of Metadata of File %d: %s\n",
				m.FileID,
				err.Error())
			return nil, err
		}

		results = append(results, m)
	}

	return results, nil
} // func (db *Database) MetaGetAll() ([]*model.FileMeta, error)
