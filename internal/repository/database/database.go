package database

import (
	"fmt"
	"time"

	"github.com/go-goe/goe"
	"github.com/go-goe/postgres"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
)

type Database struct {
	Payment *model.Payment
	*goe.DB
}

func NewDatabase(host, port, migrate string) (*Database, error) {
	dns := fmt.Sprintf("user=postgres password=postgres host=%v port=%v database=postgres", host, port)
	var db *Database
	var err error
	for range 5 {
		db, err = goe.Open[Database](postgres.Open(dns, postgres.Config{}))
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return nil, err
	}

	if migrate == "1" {
		err = goe.AutoMigrate(db)
		if err != nil {
			return nil, err
		}
	}

	return db, err
}
