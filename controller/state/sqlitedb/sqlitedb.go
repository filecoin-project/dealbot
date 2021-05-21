package sqlitedb

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	// include sqlite driver
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const serializationErrorRetryDelay = 30 * time.Millisecond

// PostgresDB is a sql.DB with reconnect functionality.
type SqliteDB struct {
	*sql.DB
	dbFile string
}

// New creates a new PostgresDB instance.
func New(dbFile string) *SqliteDB {
	return &SqliteDB{
		dbFile: dbFile,
	}
}

func (db *SqliteDB) Connect() error {
	if db.DB == nil {
		// Need to connect to database
		var err error
		if db.DB, err = sql.Open("sqlite", db.dbFile); err != nil {
			return fmt.Errorf("cannot open database: %s", err)
		}
	}
	return nil
}

func (db *SqliteDB) Name() string {
	return "sqlite"
}

func (db *SqliteDB) SqlDB() *sql.DB {
	return db.DB
}

// RetryableError determines if a transaction should be retried after failing
// due to the specified error.  When true, sleeps for a short random time
// before returning.
func (db *SqliteDB) RetryableError(err error) bool {
	if e, ok := err.(*sqlite.Error); ok {
		switch e.Code() {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_INTERRUPT:
			time.Sleep(time.Duration(rand.Int63n(int64(serializationErrorRetryDelay + 1))))
			return true
		}
	}
	return false
}
