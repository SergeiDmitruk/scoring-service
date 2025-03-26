package server

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/scoring-service/internal/middleware"
	"github.com/scoring-service/internal/storage"
)

func Init(address string) error {
	storage := storage.GetPgStorage()

	r := chi.NewRouter()
	h := NewHandler(storage)
	r.Use(middleware.LoggerMiddleware)
	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", h.Register)
		r.Post("/api/user/login", h.Login)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware) // Применяем мидлвейр только здесь
		r.Get("/api/test", h.Test)
		r.Get("/api/user/orders", h.GetUserOrders)
		r.Get("/api/user/withdrawals", h.GetUserWithdrawals)
		r.Get("/api/user/balance", h.GetUserBalance)
	})

	return http.ListenAndServe(address, r)
}

// func InitServer(address, secretKey string) error {

// 	storage := storage.GetPgStorage()

// 	r := chi.NewRouter()

// 	h := NewHandler(storage)
// 	r.Get("/", mh.GetMetrics)
// 	r.Get("/ping", mh.PingDBHandler)
// 	r.Post("/updates/", mh.UpdateAll)
// 	r.Route("/value/", func(r chi.Router) {
// 		r.Post("/", mh.GetMetricValueJSON)
// 		r.Get("/{metric_type}/{name}", mh.GetMetricValue)
// 	})
// 	r.Route("/update/", func(r chi.Router) {
// 		r.Post("/", mh.UpdateJSON)
// 		r.Post("/{metric_type}/{name}/{value}", mh.Update)
// 	})

// 	logger.Log.Sugar().Infoln("Server start on", address)
// 	return http.ListenAndServe(address, r)
// }
