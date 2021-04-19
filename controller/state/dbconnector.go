package state

import "database/sql"

type DBConnector interface {
	Connect() error
	SqlDB() *sql.DB
}
