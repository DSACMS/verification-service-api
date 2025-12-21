package config

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type OtlpConfig struct {
	Endpoint string `env:"ENDPOINT" envDefault:"https://localhost:4317"`
	Insecure bool   `env:"INSECURE" envDefault:"false"`
}

type OtelConfig struct {
	OtlpExporter OtlpConfig `envPrefix:"EXPORTER_OTLP_"`
}

type Config struct {
	Port string     `env:"PORT" envDefault:"8080"`
	Otel OtelConfig `envPrefix:"OTEL_"`
}

var AppConfig Config

func init() {
	var err error
	var errs error

	environment := getEnv("ENVIRONMENT", "development")
	if environment != "" {
		file := ".env" + environment + ".local"
		err = godotenv.Load(file)
		if err != nil {
			errs = errors.Join(
				errs,
				fmt.Errorf("error loading %s: %w", file, err),
			)
		}
	}

	err = godotenv.Load(".env.local")
	if err != nil {
		errs = errors.Join(
			errs,
			fmt.Errorf("error loading .env.local: %w", err),
		)
	}

	err = godotenv.Load()
	if err != nil {
		errs = errors.Join(
			errs,
			fmt.Errorf("error loading .env: %w", err),
		)
	}

	err = env.Parse(&AppConfig)
	if err != nil {
		errs = errors.Join(
			errs,
			fmt.Errorf("error parsing env: %w", err),
		)
	}

	if errs != nil {
		panic(errs)
	}
}

func getEnv(key, fallback string) string {
	// returns value of associated env key
	value := os.Getenv(key)

	if value != "" {
		return value
	}

	log.Printf("No env variable matching %v found... using fallback", key)

	return fallback
}
