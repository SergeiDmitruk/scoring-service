package models

import "time"

const (
	OrderNew        = "NEW"
	OrderProcessing = "PROCESSING"
	OrderInvalid    = "INVALID"
	OrderProcessed  = "PROCESSED"
)

type Order struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}
type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         int       `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
type User struct {
	Balance
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
}
type AccrualResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}
type Withdraw struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
