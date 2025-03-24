package storage

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose"
	"github.com/scoring-service/pkg/logger"
)

var conn *sql.DB

func InitDB(dsn string) error {
	var err error

	conn, err = sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("ошибка подключения к БД: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(conn, "../../migrations"); err != nil {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}

	logger.Log.Sugar().Info("Подключение к БД успешно")
	return nil
}
