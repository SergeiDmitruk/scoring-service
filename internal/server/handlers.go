package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/internal/service"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
)

//go:generate go tool mockery --inpackage --name=Service --filename=serviceinterface_test.go --with-expecter
type Service interface {
	ReagisterUser(ctx context.Context, user *models.User) error
	AuthorizeUser(ctx context.Context, user *models.User) error
	UserExist(ctx context.Context, login string) (bool, error)
	GetUserOrders(ctx context.Context, id int) ([]models.Order, error)
	GetUserWithdrawals(ctx context.Context, id int) ([]models.Withdrawal, error)
	GetUserBalance(ctx context.Context, id int) (models.Balance, error)
	CreateOrder(ctx context.Context, userID int, orderNum string) service.CreateStatus
	CreateWithdraw(ctx context.Context, userID int, withdraw models.Withdraw) service.CreateStatus
}

type Handler struct {
	serv Service
}

func NewHandler(service Service) *Handler {
	return &Handler{serv: service}
}
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var newUser models.User
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if newUser.Password == "" || newUser.Login == "" {
		http.Error(w, "Невалидный логин или пароль", http.StatusBadRequest)
		return
	}
	alreadyExist, err := h.serv.UserExist(r.Context(), newUser.Login)
	if err != nil {
		http.Error(w, "Ошибка при создании пользователя", http.StatusInternalServerError)
		return
	}
	if alreadyExist {
		http.Error(w, "Логин уже занят", http.StatusConflict)
		return
	}

	if err := h.serv.ReagisterUser(r.Context(), &newUser); err != nil {
		http.Error(w, "Ошибка регистрации клиента", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateJWT(&newUser)
	if err != nil {
		http.Error(w, "Ошибка при генерации токена", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Пользователь успешно зарегистрирован и аутентифицирован"))
}
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.User
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if req.Password == "" || req.Login == "" {
		http.Error(w, "Невалидный логин или пароль", http.StatusBadRequest)
		return
	}
	if err := h.serv.AuthorizeUser(r.Context(), &req); err != nil {
		http.Error(w, "Неверная пара логин/пароль", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(&req)
	if err != nil {
		http.Error(w, "Ошибка при генерации токена", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Пользователь успешно аутентифицирован"))
}
func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.serv.GetUserOrders(ctx, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(orders)
}
func (h *Handler) GetUserWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.serv.GetUserWithdrawals(ctx, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(withdrawals)
}
func (h *Handler) GetUserBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	balance, err := h.serv.GetUserBalance(ctx, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(balance)
}
func (h *Handler) PostOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	orderNum := strings.TrimSpace(string(body))

	status := h.serv.CreateOrder(r.Context(), userID, orderNum)

	switch status {
	case service.StatusOK:
		w.WriteHeader(http.StatusAccepted)
	case service.StatusAlreadyExist:
		w.WriteHeader(http.StatusOK)
	case service.StatusConflict:
		http.Error(w, "order already exists for another user", http.StatusConflict)
		return
	case service.StatusInvalid:
		http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		return
	case service.StatusError:
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	default:
		logger.Log.Sugar().Error("Unknown status ", status)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.Withdraw
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}
	if req.Sum <= 0 {
		http.Error(w, "sum must be greater than zero", http.StatusBadRequest)
		return
	}

	status := h.serv.CreateWithdraw(ctx, userID, req)
	switch status {
	case service.StatusOK:
		w.WriteHeader(http.StatusOK)
	case service.StatusAlreadyExist:
		w.WriteHeader(http.StatusOK)
	case service.StatusConflict:
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		return
	case service.StatusInvalid:
		http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		return
	case service.StatusError:
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	default:
		logger.Log.Sugar().Error("Unknown status ", status)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

}
