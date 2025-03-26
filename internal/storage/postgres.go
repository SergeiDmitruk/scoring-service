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

func (db *PgStorage) CreateUser(ctx context.Context, user *models.User) error {
	var userID int
	query := "INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id"
	err := db.QueryRowContext(ctx, query, user.Login, user.Password).Scan(&userID)
	if err != nil {
		logger.Log.Error("Ошибка при создании пользователя")
		return fmt.Errorf("ошибка при создании пользователя: %w", err)
	}
	user.ID = userID
	return nil
}
func (db *PgStorage) GetUserOrders(ctx context.Context, userID int) ([]models.Order, error) {
	var orders []models.Order

	query := `
        SELECT number, status, accrual, uploaded_at
        FROM orders
        WHERE user_id = $1
        ORDER BY uploaded_at DESC
    `
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error(err.Error())
		return nil, fmt.Errorf("ошибка при получении заказов: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			logger.Log.Error(err.Error())
			return nil, fmt.Errorf("ошибка при чтении данных заказа: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}

	return orders, nil
}
func (db *PgStorage) GetUserWithdrawals(ctx context.Context, userID int) ([]models.Withdrawal, error) {
	var withdrawals []models.Withdrawal

	query := `
		SELECT order_number, sum, uploaded_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY uploaded_at DESC
	`
	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var withdrawal models.Withdrawal
		if err := rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			logger.Log.Error(err.Error())
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err := rows.Err(); err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}

	return withdrawals, nil
}
func (db *PgStorage) GetUserBalance(ctx context.Context, userID int) (models.Balance, error) {
	var balance models.Balance

	query := `
		SELECT 
			COALESCE(current_balance, 0),
			COALESCE(withdrawn, 0)
		FROM users
		WHERE id = $1
	`
	err := db.QueryRowContext(ctx, query, userID).Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		logger.Log.Error(err.Error())
		return balance, err
	}

	return balance, nil
}
func (db *PgStorage) CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
