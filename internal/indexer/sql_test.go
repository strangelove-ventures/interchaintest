package indexer

import "database/sql"

func emptyDB() *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	return db
}

func migratedDB() *sql.DB {
	db := emptyDB()
	if err := Migrate(db); err != nil {
		panic(err)
	}
	return db
}
