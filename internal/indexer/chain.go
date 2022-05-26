package indexer

import "database/sql"

// TODO
type Chain struct {
	identifier string
	db         *sql.DB
}
