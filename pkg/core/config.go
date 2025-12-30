package core

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

type Config struct {
	Environment string     `env:"ENVIRONMENT" envDefault:"development"`
	Port        string     `env:"PORT" envDefault:"8080"`
	Otel        OtelConfig `envPrefix:"OTEL_"`
	SkipAuth    bool       `env:"SKIP_AUTH" envDefault:"false"`
}

func loadEnv(filename string) error {
	err := godotenv.Load(filename)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("error loading file %s: %w", filename, err)
}

func NewConfig() (Config, error) {
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

	config := Config{}
	err = env.Parse(&config)
	if err != nil {
		errs = errors.Join(
			errs,
			fmt.Errorf("error parsing env: %w", err),
		)
	}

	return config, errs
}

func getEnv(key, fallback string) string {
	// returns value of associated env key
	value := os.Getenv(key)

	if value != "" {
		return value
	}

	return fallback
}

func (c Config) IsProd() bool {
	return c.Environment == "production"
}
