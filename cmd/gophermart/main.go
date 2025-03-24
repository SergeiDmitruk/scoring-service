package main

import (
	"flag"
	"log"
	"os"

	"github.com/scoring-service/internal/server"
	"github.com/scoring-service/internal/storage"
	"github.com/scoring-service/pkg/logger"
)

var (
	runAddress           string
	databaseURI          string
	accrualSystemAddress string
)

func initConfig() {
	flag.StringVar(&runAddress, "a", getEnv("RUN_ADDRESS", "localhost:8080"), "Адрес и порт запуска сервиса")
	flag.StringVar(&databaseURI, "d", getEnv("DATABASE_URI", "postgres://user:password@localhost:5432/gophermart?sslmode=disable"), "Адрес подключения к базе данных")
	flag.StringVar(&accrualSystemAddress, "r", getEnv("ACCRUAL_SYSTEM_ADDRESS", "http://accrual-system.local"), "Адрес системы расчёта начислений")

	flag.Parse()
}

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func main() {
	initConfig()
	if err := logger.Init("info"); err != nil {
		log.Fatal(err)
	}

	logger.Log.Sugar().Info("Сервис запускается на адресе: %s", runAddress)
	logger.Log.Sugar().Info("Подключение к базе данных: %s", databaseURI)
	logger.Log.Sugar().Info("Адрес системы расчёта начислений: %s", accrualSystemAddress)
	if err := storage.InitDB(databaseURI); err != nil {
		logger.Log.Sugar().Fatal(err)
	}
	if err := server.Init(runAddress); err != nil {
		logger.Log.Sugar().Fatal(err)
	}
}
