package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type OtlpConfig struct {
	Endpoint string `env:"ENDPOINT" envDefault:"localhost:4317"`
	Insecure bool   `env:"INSECURE" envDefault:"false"`
}

type OtelConfig struct {
	OtlpExporter OtlpConfig `envPrefix:"EXPORTER_OTLP_"`
}

type RedisConfig struct {
	Addr string `env:"ADDR" envDefault:"localhost:6379"`
}

type Config struct {
	Environment string      `env:"ENVIRONMENT" envDefault:"development"`
	Port        string      `env:"PORT" envDefault:"8080"`
	Redis       RedisConfig `envPrefix:"REDIS_"`
	Otel        OtelConfig  `envPrefix:"OTEL_"`
}

var AppConfig Config

func loadEnv(filename string) error {
	err := godotenv.Load(filename)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("error loading file %s: %w", filename, err)
}

func init() {
	var err error
	var errs error

	environment := getEnv("ENVIRONMENT", "development")
	if environment != "" {
		file := ".env" + environment + ".local"
		err = loadEnv(file)
		if err != nil {
			errs = errors.Join(
				errs,
				fmt.Errorf("error loading %s: %w", file, err),
			)
		}
	}

	err = loadEnv(".env.local")
	if err != nil {
		errs = errors.Join(
			errs,
			fmt.Errorf("error loading .env.local: %w", err),
		)
	}

	err = loadEnv(".env")
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

	return fallback
}

func IsProd() bool {
	return AppConfig.Environment == "production"
}
