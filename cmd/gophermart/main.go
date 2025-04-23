package main

import (
	"flag"
	"log"
	"net/url"
	"os"

	"github.com/scoring-service/internal/server"
	"github.com/scoring-service/internal/service"
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
	flag.StringVar(&accrualSystemAddress, "r", getEnv("ACCRUAL_SYSTEM_ADDRESS", "localhost:9090"), "Адрес системы расчёта начислений")

	flag.Parse()
}

func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}
func validateURL(u string) error {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		return err
	}
	return nil
}
func main() {
	initConfig()
	if err := logger.Init("info"); err != nil {
		log.Fatal(err)
	}
	if err := validateURL("http://" + runAddress); err != nil {
		logger.Log.Sugar().Fatal("Некорректный адрес запуска сервиса: ", err)
	}

	if err := validateURL(databaseURI); err != nil {
		logger.Log.Sugar().Fatal("Некорректный адрес к базе данных: ", err)
	}

	if err := validateURL(accrualSystemAddress); err != nil {
		logger.Log.Sugar().Fatal("Некорректный адрес к сервису расчета баллов: ", err)
	}
	logger.Log.Sugar().Info("Сервис запускается на адресе:", runAddress)
	logger.Log.Sugar().Info("Подключение к базе данных:", databaseURI)
	logger.Log.Sugar().Info("Адрес системы расчёта начислений:", accrualSystemAddress)
	storage, err := storage.InitDB(databaseURI)
	if err != nil {
		logger.Log.Sugar().Fatal(err)
	}
	serv := service.NewAccrualService(storage, accrualSystemAddress)
	service.GetQueueManager(serv)
	if err := server.Init(runAddress, serv); err != nil {
		logger.Log.Sugar().Fatal(err)
	}

	storage.CloseDB()
}
