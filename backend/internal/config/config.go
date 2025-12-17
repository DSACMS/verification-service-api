package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
}

var AppConfig Config

func LoadEnv() {
	err := godotenv.Load()

	if err != nil {
		log.Println("No .env file found... Using system env vars")
	}

	AppConfig = Config{
		Port:        getEnv("PORT", "8000"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	// returns value of associated env key
	value := os.Getenv(key)

	if value != "" {
		return value 
	}

	log.Printf("No env variable matching %v found... using fallback", value)

	return fallback
}
