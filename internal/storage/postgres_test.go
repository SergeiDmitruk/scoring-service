package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/scoring-service/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserOrders(t *testing.T) {
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	store := PgStorage{DB: mockDB}
	ctx := context.Background()

	mock.ExpectQuery("SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = \\$1 ORDER BY uploaded_at DESC").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
			AddRow("123", "NEW", 10.5, time.Now()))

	orders, err := store.GetUserOrders(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, "123", orders[0].Number)

	mock.ExpectQuery("SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = \\$1 ORDER BY uploaded_at DESC").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}))

	orders, err = store.GetUserOrders(ctx, 2)
	assert.NoError(t, err)
	assert.Empty(t, orders)

	mock.ExpectQuery("SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = \\$1 ORDER BY uploaded_at DESC").
		WithArgs(3).
		WillReturnError(errors.New("db error"))

	orders, err = store.GetUserOrders(ctx, 3)
	assert.Error(t, err)
	assert.Nil(t, orders)
}

func TestCreateUser(t *testing.T) {
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	store := PgStorage{DB: mockDB}
	ctx := context.Background()

	user := &models.User{
		Login:    "testuser",
		Password: "hashedpassword",
	}

	mock.ExpectQuery("INSERT INTO users \\(login, password_hash\\) VALUES \\(\\$1, \\$2\\) RETURNING id").
		WithArgs(user.Login, user.Password).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := store.CreateUser(ctx, user)
	assert.NoError(t, err)
	assert.Equal(t, 1, user.ID)

	mock.ExpectQuery("^INSERT INTO users \\(login, password_hash\\) VALUES \\($1, $2\\) RETURNING id$").
		WithArgs(user.Login, user.Password).
		WillReturnError(errors.New("db error"))

	err = store.CreateUser(ctx, user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка при создании пользователя")
}
func TestGetUserByLogin(t *testing.T) {
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	store := PgStorage{DB: mockDB}
	ctx := context.Background()

	login := "testuser"

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, login, password_hash FROM users WHERE login = \\$1").
			WithArgs(login).
			WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(1, login, "hashedpassword"))

		user, err := store.GetUserByLogin(ctx, login)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, 1, user.ID)
		assert.Equal(t, login, user.Login)
		assert.Equal(t, "hashedpassword", user.Password)
	})

	t.Run("NoRows", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, login, password_hash FROM users WHERE login = \\$1").
			WithArgs(login).
			WillReturnError(sql.ErrNoRows)

		user, err := store.GetUserByLogin(ctx, login)

		assert.NoError(t, err)
		assert.Nil(t, user)
	})

	t.Run("Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, login, password_hash FROM users WHERE login = \\$1").
			WithArgs(login).
			WillReturnError(errors.New("db error"))

		user, err := store.GetUserByLogin(ctx, login)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "db error")
	})
}
func TestGetUserWithdrawals(t *testing.T) {
	mockDB, mock, _ := sqlmock.New()
	defer mockDB.Close()

	store := PgStorage{DB: mockDB}
	ctx := context.Background()

	userID := 1

	t.Run("Success", func(t *testing.T) {
		t1, _ := time.Parse("2006-01-02 15:04:05", "2025-03-30 12:00:00")
		t2, _ := time.Parse("2006-01-02 15:04:05", "2025-03-29 12:00:00")

		mock.ExpectQuery(regexp.QuoteMeta("SELECT order_number, sum, uploaded_at FROM withdrawals WHERE user_id = $1 ORDER BY uploaded_at DESC")).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "uploaded_at"}).
				AddRow("order1", 100.0, t1).
				AddRow("order2", 200.0, t2))

		withdrawals, err := store.GetUserWithdrawals(ctx, userID)

		assert.NoError(t, err)
		assert.Len(t, withdrawals, 2)
		assert.Equal(t, "order1", withdrawals[0].Order)
		assert.Equal(t, 100.0, withdrawals[0].Sum)
		assert.Equal(t, t1, withdrawals[0].ProcessedAt)
		assert.Equal(t, "order2", withdrawals[1].Order)
		assert.Equal(t, 200.0, withdrawals[1].Sum)
		assert.Equal(t, t2, withdrawals[1].ProcessedAt)
	})

	t.Run("NoWithdrawals", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT order_number, sum, uploaded_at
			FROM withdrawals
			WHERE user_id = $1
			ORDER BY uploaded_at DESC`)).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "uploaded_at"}))

		withdrawals, err := store.GetUserWithdrawals(ctx, userID)

		assert.NoError(t, err)
		assert.Len(t, withdrawals, 0)
	})

	t.Run("QueryError", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT order_number, sum, uploaded_at
			FROM withdrawals
			WHERE user_id = $1
			ORDER BY uploaded_at DESC`)).
			WithArgs(userID).
			WillReturnError(errors.New("query error"))

		withdrawals, err := store.GetUserWithdrawals(ctx, userID)

		assert.Error(t, err)
		assert.Nil(t, withdrawals)
		assert.Contains(t, err.Error(), "query error")
	})

}
func TestGetUserBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}

	ctx := context.Background()
	userID := 1

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT 
				COALESCE(current_balance, 0),
				COALESCE(withdrawn, 0)
			FROM users
			WHERE id = $1`)).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance", "withdrawn"}).
				AddRow(100.0, 50.0))

		balance, err := store.GetUserBalance(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, 100.0, balance.Current)
		assert.Equal(t, 50.0, balance.Withdrawn)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT 
				COALESCE(current_balance, 0),
				COALESCE(withdrawn, 0)
			FROM users
			WHERE id = $1`)).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		balance, err := store.GetUserBalance(ctx, userID)

		assert.Error(t, err)
		assert.Equal(t, 0.0, balance.Current)
		assert.Equal(t, 0.0, balance.Withdrawn)
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}
func TestSaveOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}

	ctx := context.Background()
	userID := 1
	order := &models.Order{
		Number:  "123456789",
		Status:  "NEW",
		Accrual: 10.5,
	}

	t.Run("SuccessInsert", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO orders (user_id, number, status, accrual, uploaded_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (number) DO UPDATE
			SET status = $3, accrual = $4, uploaded_at = NOW();
		`)).
			WithArgs(userID, order.Number, order.Status, sql.NullFloat64{Float64: order.Accrual, Valid: true}).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.SaveOrder(ctx, userID, order)

		assert.NoError(t, err)
	})

	t.Run("DatabaseError", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO orders (user_id, number, status, accrual, uploaded_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (number) DO UPDATE
			SET status = $3, accrual = $4, uploaded_at = NOW();
		`)).
			WithArgs(userID, order.Number, order.Status, sql.NullFloat64{Float64: order.Accrual, Valid: true}).
			WillReturnError(sql.ErrConnDone)

		err := store.SaveOrder(ctx, userID, order)
		assert.Error(t, err)
		assert.Equal(t, sql.ErrConnDone, err)
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}
func TestUpdateOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}

	ctx := context.Background()
	accrual := &models.AccrualResponse{
		Order:   "123456789",
		Status:  "PROCESSED",
		Accrual: 15.75,
	}

	t.Run("SuccessUpdate", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
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
		`)).
			WithArgs(accrual.Order, accrual.Status, sql.NullFloat64{Float64: accrual.Accrual, Valid: accrual.Accrual > 0}).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := store.UpdateOrder(ctx, accrual)

		assert.NoError(t, err)
	})

	t.Run("DatabaseError", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta(`
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
		`)).
			WithArgs(accrual.Order, accrual.Status, sql.NullFloat64{Float64: accrual.Accrual, Valid: accrual.Accrual > 0}).
			WillReturnError(sql.ErrConnDone)

		err := store.UpdateOrder(ctx, accrual)

		assert.Error(t, err)
		assert.Equal(t, sql.ErrConnDone, err)
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}
func TestIsOrderExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}

	ctx := context.Background()
	orderNum := "123456789"

	t.Run("OrderExists", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
            SELECT user_id 
            FROM orders
            WHERE number = $1
        ;`)).
			WithArgs(orderNum).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(42))

		userID, err := store.IsOrderExists(ctx, orderNum)

		assert.NoError(t, err)
		assert.Equal(t, 42, userID)
	})

	t.Run("OrderDoesNotExist", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
            SELECT user_id 
            FROM orders
            WHERE number = $1
        ;`)).
			WithArgs(orderNum).
			WillReturnError(sql.ErrNoRows)

		userID, err := store.IsOrderExists(ctx, orderNum)

		assert.NoError(t, err)
		assert.Equal(t, 0, userID)
	})

	t.Run("DatabaseError", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
            SELECT user_id 
            FROM orders
            WHERE number = $1
        ;`)).
			WithArgs(orderNum).
			WillReturnError(sql.ErrConnDone)

		userID, err := store.IsOrderExists(ctx, orderNum)

		assert.Error(t, err)
		assert.Equal(t, 0, userID)
		assert.Equal(t, sql.ErrConnDone, err)
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}
func TestWithdraw(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}
	ctx := context.Background()
	userID := 1
	orderNum := "123456789"
	amount := 100.0

	t.Run("Success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT current_balance FROM users WHERE id = $1 FOR UPDATE;
		`)).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance"}).AddRow(200.0))

		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO withdrawals (user_id, order_number, sum, uploaded_at)
			VALUES ($1, $2, $3, NOW());
		`)).
			WithArgs(userID, orderNum, amount).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectExec(regexp.QuoteMeta(`
			UPDATE users
			SET current_balance = current_balance - $1,
			withdrawn = withdrawn + $1
			WHERE id = $2;
		`)).
			WithArgs(amount, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		mock.ExpectCommit()

		err := store.Withdraw(ctx, userID, orderNum, amount)
		assert.NoError(t, err)
	})

	t.Run("InsufficientFunds", func(t *testing.T) {
		mock.ExpectBegin()

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT current_balance FROM users WHERE id = $1 FOR UPDATE;
		`)).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance"}).AddRow(50.0))

		mock.ExpectRollback()

		err := store.Withdraw(ctx, userID, orderNum, amount)
		assert.Error(t, err)
		assert.Equal(t, "недостаточно средств", err.Error())
	})

	t.Run("QueryError", func(t *testing.T) {
		mock.ExpectBegin()

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT current_balance FROM users WHERE id = $1 FOR UPDATE;
		`)).
			WithArgs(userID).
			WillReturnError(errors.New("db error"))

		mock.ExpectRollback()

		err := store.Withdraw(ctx, userID, orderNum, amount)
		assert.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})

	t.Run("TransactionFailure", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrTxDone)

		err := store.Withdraw(ctx, userID, orderNum, amount)
		assert.Error(t, err)
		assert.Equal(t, sql.ErrTxDone, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
func TestGetPendingOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &PgStorage{DB: db}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"order_num"}).
			AddRow("ORD001").
			AddRow("ORD002").
			AddRow("ORD003")

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT order_num
			FROM orders
			WHERE status IN ('NEW', 'PROCESSING')
		`)).WillReturnRows(rows)

		result, err := store.GetPendingOrders(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"ORD001", "ORD002", "ORD003"}, result)
	})

	t.Run("QueryError", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT order_num
			FROM orders
			WHERE status IN ('NEW', 'PROCESSING')
		`)).WillReturnError(errors.New("query failed"))

		result, err := store.GetPendingOrders(ctx)
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("ScanError", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"order_num"}).
			AddRow(nil)

		mock.ExpectQuery(regexp.QuoteMeta(`
			SELECT order_num
			FROM orders
			WHERE status IN ('NEW', 'PROCESSING')
		`)).WillReturnRows(rows)

		result, err := store.GetPendingOrders(ctx)
		require.Error(t, err)
		require.Nil(t, result)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
