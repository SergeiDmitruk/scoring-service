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

func InitDB(dsn string) (*PgStorage, error) {
	var err error

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return &PgStorage{}, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := db.Ping(); err != nil {
		return &PgStorage{}, fmt.Errorf("ошибка пинга БД: %w", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return &PgStorage{}, err
	}

	if err := goose.Up(db, "../../internal/migrations"); err != nil {
		return &PgStorage{}, fmt.Errorf("ошибка применения миграций: %w", err)
	}

	logger.Log.Sugar().Info("Подключение к БД успешно")
	return &PgStorage{DB: db}, nil
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
		var accrual sql.NullFloat64

		if err := rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt); err != nil {
			logger.Log.Error(err.Error())
			return nil, fmt.Errorf("ошибка при чтении данных заказа: %w", err)
		}

		if accrual.Valid {
			order.Accrual = accrual.Float64
		} else {
			order.Accrual = 0
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

func (db *PgStorage) SaveOrder(ctx context.Context, user int, order *models.Order) error {
	_, err := db.ExecContext(ctx, `
        INSERT INTO orders (user_id, number, status, accrual, uploaded_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (number) DO UPDATE
        SET status = $3, accrual = $4, uploaded_at = NOW();
    `, user, order.Number, order.Status, sql.NullFloat64{Float64: order.Accrual, Valid: true})
	if err != nil {
		logger.Log.Error(err.Error())
	}
	return err
}
func (db *PgStorage) UpdateOrder(ctx context.Context, accrual *models.AccrualResponse) error {
	_, err := db.ExecContext(ctx, `
    WITH updated_order AS (
        UPDATE orders
        SET status = $2, accrual = $3
        WHERE number = $1
        RETURNING user_id, accrual
    )
    UPDATE users
    SET current_balance = current_balance + uo.accrual
    FROM updated_order uo
    WHERE users.id = uo.user_id AND uo.accrual > 0;
	`, accrual.Order, accrual.Status, sql.NullFloat64{Float64: accrual.Accrual, Valid: accrual.Accrual > 0})
	if err != nil {
		logger.Log.Error(err.Error())
	}
	return err
}

func (db *PgStorage) IsOrderExists(ctx context.Context, orderNum string) (int, error) {
	var userID int
	query := `
            SELECT user_id 
            FROM orders
            WHERE number = $1
        ;
    `
	err := db.QueryRowContext(ctx, query, orderNum).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		logger.Log.Error(err.Error())
		return 0, err
	}
	return userID, nil
}

func (db *PgStorage) Withdraw(ctx context.Context, userID int, order string, sum float64) error {
	tx, err := db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentBalance float64

	err = tx.QueryRowContext(ctx, `
        SELECT current_balance FROM users WHERE id = $1 FOR UPDATE;
    `, userID).Scan(&currentBalance)
	if err != nil {
		return err
	}

	if currentBalance < sum {
		return fmt.Errorf("недостаточно средств")
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO withdrawals (user_id, order_number, sum, uploaded_at)
        VALUES ($1, $2, $3, NOW());
    `, userID, order, sum)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
        UPDATE users
        SET current_balance = current_balance - $1,
            withdrawn = withdrawn + $1
        WHERE id = $2;
    `, sum, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *PgStorage) GetPendingOrders(ctx context.Context) ([]string, error) {
	var orders []string

	query := `
		SELECT number
		FROM orders
		WHERE status IN ('NEW', 'PROCESSING')
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orderNum string
		if err := rows.Scan(&orderNum); err != nil {
			logger.Log.Error(err.Error())
			return nil, err
		}
		orders = append(orders, orderNum)
	}

	if err := rows.Err(); err != nil {
		logger.Log.Error(err.Error())
		return nil, err
	}

	return orders, nil
}
