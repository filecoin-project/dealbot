package sqlitedb

import (
	"database/sql"
	"fmt"

	// include sqlite driver
	//_ "modernc.org/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

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
		if db.DB, err = sql.Open("sqlite3", db.dbFile); err != nil {
			return fmt.Errorf("cannot open database: %s", err)
		}
	}
	return nil
}

func (db *SqliteDB) SqlDB() *sql.DB {
	return db.DB
}
