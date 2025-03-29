package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/internal/service"
	"github.com/scoring-service/internal/storage"
	"github.com/scoring-service/pkg/logger"
	"github.com/scoring-service/pkg/models"
	"go.uber.org/zap"
)

type handler struct {
	storage storage.StorageInterface
}

func NewHandler(storage storage.StorageInterface) *handler {
	return &handler{storage: storage}
}
func (h *handler) Register(w http.ResponseWriter, r *http.Request) {
	var newUser models.User
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if newUser.Password == "" || newUser.Login == "" { // написать валидатор от иньекций?
		http.Error(w, "Невалидный логин или пароль", http.StatusBadRequest)
		return
	}
	user, err := h.storage.GetUserByLogin(r.Context(), newUser.Login)
	if err == nil && user != nil {
		http.Error(w, "Логин уже занят", http.StatusConflict)
		return
	}

	hashedPassword, err := auth.HashPassword(newUser.Password)
	if err != nil {
		http.Error(w, "Ошибка хеширования пароля", http.StatusInternalServerError)
		return
	}

	newUser.Password = hashedPassword

	err = h.storage.CreateUser(r.Context(), &newUser)
	if err != nil {
		http.Error(w, "Ошибка при создании пользователя", http.StatusInternalServerError)
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
func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.User
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}
	if req.Password == "" || req.Login == "" { // написать валидатор от иньекций
		http.Error(w, "Невалидный логин или пароль", http.StatusBadRequest)
		return
	}
	user, err := h.storage.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		http.Error(w, "Неверная пара логин/пароль", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.Password) {
		http.Error(w, "Неверная пара логин/пароль", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user)
	if err != nil {
		http.Error(w, "Ошибка при генерации токена", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Пользователь успешно аутентифицирован"))
}
func (h *handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.storage.GetUserOrders(ctx, userID)
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
func (h *handler) GetUserWithdrawals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.storage.GetUserWithdrawals(ctx, userID)
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
func (h *handler) GetUserBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(auth.UserIDKey).(int)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	balance, err := h.storage.GetUserBalance(ctx, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(balance)
}
func (h *handler) PostOrder(w http.ResponseWriter, r *http.Request) {
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

	if !auth.IsValidLuhn(orderNum) {
		logger.Log.Error("invalid order number format", zap.String("order", orderNum))
		if err := s.db.UpdateOrder(ctx, &models.AccrualResponse{Order: orderNumber, Status: models.OrderInvalid}); err != nil {
			logger.Log.Error(err.Error())
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	realUserID, err := h.storage.IsOrderExists(r.Context(), orderNum)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if realUserID == 0 {
		newOrder := models.Order{
			Number: orderNum,
			Status: models.OrderNew,
		}
		err := h.storage.SaveOrder(r.Context(), userID, &newOrder)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	} else {
		if userID != realUserID {
			http.Error(w, "order already exists for another user", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	service.GetQueueManager().JobQueue <- service.OrderJob{UserID: userID, OrderNum: orderNum}

}
func (h *handler) WithdrawBalance(w http.ResponseWriter, r *http.Request) {

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
	if !auth.IsValidLuhn(req.Order) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}
	if req.Sum <= 0 {
		http.Error(w, "sum must be greater than zero", http.StatusBadRequest)
		return
	}

	balance, err := h.storage.GetUserBalance(ctx, userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if balance.Current < req.Sum {
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		return
	}

	err = h.storage.Withdraw(ctx, userID, req.Order, req.Sum)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
