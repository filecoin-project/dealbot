package postgresdb

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// PostgresDB is a sql.DB with reconnect functionality.
type PostgresDB struct {
	*sql.DB
	dbConnStr string
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

func (db *PostgresDB) SqlDB() *sql.DB {
	return db.DB
}
