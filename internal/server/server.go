package server

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/scoring-service/internal/middleware"
)

func Init(address string, service Service) error {
	r := chi.NewRouter()
	h := NewHandler(service)
	r.Use(middleware.LoggerMiddleware)
	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", h.Register)
		r.Post("/api/user/login", h.Login)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)
		r.Use(middleware.GzipMiddleware)
		r.Get("/api/user/orders", h.GetUserOrders)
		r.Post("/api/user/orders", h.PostOrder)
		r.Get("/api/user/withdrawals", h.GetUserWithdrawals)
		r.Get("/api/user/balance", h.GetUserBalance)
		r.Post("/api/user/balance/withdraw", h.Withdraw)

	})

	return http.ListenAndServe(address, r)
}
