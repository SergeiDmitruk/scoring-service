package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
)

type PgStorage struct {
	*sql.DB
}

var pgStorageInstance PgStorage

func GetPgStorage() *PgStorage {
	return &pgStorageInstance
}

func InitDB(dsn string) error {
	var err error

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ошибка пинга БД: %w", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, "../../migrations"); err != nil {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}
	pgStorageInstance.DB = db

	logger.Log.Sugar().Info("Подключение к БД успешно")
	return nil
}

func (db *PgStorage) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {
	var user models.User

	query := "SELECT id, login, password_hash FROM users WHERE login = $1"
	row := db.QueryRowContext(ctx, query, login)

	err := row.Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (db *PgStorage) CreateUser(ctx context.Context, user models.User) error {
	query := "INSERT INTO users (login, password_hash) VALUES ($1, $2)"
	_, err := db.ExecContext(ctx, query, user.Login, user.Password)
	if err != nil {
		logger.Log.Error(err.Error())
	}
	return err
}

func (db *PgStorage) CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
