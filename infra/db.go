package infra

import (
	"database/sql"
	"os"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

var (
	once sync.Once
	db   *sql.DB
)

func DB() *sql.DB {
	once.Do(func() {
		var err error
		dsn := os.Getenv("DB_DSN")
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			panic(err)
		}
		db.SetMaxOpenConns(100)
		db.SetMaxIdleConns(10)
	})
	return db
}
