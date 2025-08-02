package database

import (
	"github.com/go-goe/goe"
	"github.com/go-goe/postgres"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
)

type Database struct {
	Payment *model.Payment
	*goe.DB
}

func NewDatabase() (*Database, error) {
	dns := "user=postgres password=postgres host=localhost port=5433 database=postgres"
	db, err := goe.Open[Database](postgres.Open(dns, postgres.Config{}))
	if err != nil {
		return nil, err
	}

	err = goe.AutoMigrate(db)
	if err != nil {
		return nil, err
	}

	return db, err
}
