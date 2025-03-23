package server

import (
	"net/http"
)

func Init(address string) error {
	return http.ListenAndServe(address, nil)
}
