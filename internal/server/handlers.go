package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/scoring-service/internal/auth"
	"github.com/scoring-service/internal/storage"
	"github.com/scoring-service/pkg/models"
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
	if newUser.Password == "" || newUser.Login == "" { // написать валидатор от иньекций
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

	err = h.storage.CreateUser(r.Context(), newUser)
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
func (h *handler) Test(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Иди на хуй"))
}
