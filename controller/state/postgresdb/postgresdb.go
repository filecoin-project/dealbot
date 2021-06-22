package postgresdb

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"sync"
	"syscall"
	"time"

	// include postgres driver
	"github.com/lib/pq"
)

// PostgresDB is a sql.DB with reconnect functionality.
type PostgresDB struct {
	*sql.DB
	dbConnStr string
	mutex     sync.Mutex
}

// PostgresConfig contains config for postgres connection
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

const serializationErrorRetryDelay = 30 * time.Millisecond

func (pgcfg PostgresConfig) String() string {
	if pgcfg.Host == "" {
		pgcfg.Host = os.Getenv("PGHOST")
	}
	if pgcfg.Port == "" {
		pgcfg.Port = os.Getenv("PGPORT")
	}
	if pgcfg.User == "" {
		pgcfg.User = os.Getenv("PGUSER")
	}
	if pgcfg.Password == "" {
		pgcfg.Password = os.Getenv("PGPASSWORD")
	}
	if pgcfg.Database == "" {
		pgcfg.Database = os.Getenv("PGDATABASE")
	}
	if pgcfg.SSLMode == "" {
		pgcfg.SSLMode = os.Getenv("PGSSLMODE")
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		pgcfg.Host, pgcfg.Port, pgcfg.User, pgcfg.Password, pgcfg.Database, pgcfg.SSLMode)
}

// New creates a new PostgresDB instance.
func New(connString string) *PostgresDB {
	return &PostgresDB{
		dbConnStr: connString,
	}
}

// Connect checks database connection and connects if not connected.
func (db *PostgresDB) Connect() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	var err error
	// If connection exists, ping to check it
	if db.DB != nil {
		if err = db.DB.Ping(); err != nil {
			// Ping failed, close and prep for reconnect
			db.DB.Close()
			db.DB = nil
		}
	}
	if db.DB == nil {
		// Need to connect to database
		if db.DB, err = sql.Open("postgres", db.dbConnStr); err != nil {
			return fmt.Errorf("cannot connect to database: %s", err)
		}
		if err = db.DB.Ping(); err != nil {
			return fmt.Errorf("cannot ping database: %s", err)
		}
	}
	return nil
}

func (db *PostgresDB) Name() string {
	return "postgres"
}

func (db *PostgresDB) SqlDB() *sql.DB {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.DB
}

// RetryableError determines if a transaction should be retried after failing
// due to the specified error.  When true, sleeps for a short random time
// before returning.
func (db *PostgresDB) RetryableError(err error) bool {
	const (
		errSerializationFailure  = "40001"
		errDeadlockDetected      = "40P01"
		errInsufficientResources = "53000"
		errCannotConnectNow      = "57P03"
	)

	var e *pq.Error
	if errors.As(err, &e) {
		switch string(e.Code) {
		case errSerializationFailure, errDeadlockDetected, errCannotConnectNow, errInsufficientResources:
			time.Sleep(time.Duration(rand.Int63n(int64(serializationErrorRetryDelay + 1))))
			return true
		}
	}
	if err == driver.ErrBadConn {
		if connErr := db.Connect(); connErr != nil {
			// If failed to reconnect, wait before retrying in hopes that the
			// database will be online by next retry
			time.Sleep(time.Second)
		}
		return true
	}
	return retryableError(err)
}

func retryableError(err error) bool {
	switch e := err.(type) {
	case *net.DNSError:
		return e.Temporary()
	case *net.OpError:
		if e.Temporary() {
			return true
		}
		return retryableError(e.Err)
	case *url.Error:
		if e.Temporary() {
			return true
		}
		return retryableError(e.Err)
	case *os.SyscallError:
		return retryableError(e.Err)
	case syscall.Errno:
		return e == syscall.EAGAIN ||
			e == syscall.ECONNABORTED ||
			e == syscall.ECONNRESET ||
			e == syscall.ENETRESET ||
			e == syscall.ENODATA
	}
	return false
}
