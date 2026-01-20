package core

import (
	"errors"
	"fmt"
)

const (
	defaultConfigEnvironment = "development"
	defaultConfigPort        = 8000
	defaultSkipAuth          = false

	defaultOtelDisable          = false
	defaultOTLPExporterEndpoint = "localhost:4317"
	defaultOTLPInsecure         = false

	defaultCognitoRegion      = "us-east-1"
	defaultCognitoUserPoolID  = "UNSET"
	defaultCognitoAppClientID = "UNSET"

	defaultRedisAddr     = "localhost:6379"
	defaultRedisPassword = ""
	defaultRedisDB       = 0
)

func DefaultConfig() Config {
	return Config{
		Environment: defaultConfigEnvironment,
		Port:        defaultConfigPort,
		SkipAuth:    defaultSkipAuth,
		Otel: OtelConfig{
			Disable: defaultOtelDisable,
			OtlpExporter: OtlpConfig{
				Endpoint: defaultOTLPExporterEndpoint,
				Insecure: defaultOTLPInsecure,
			},
		},
		Cognito: CognitoConfig{
			Region:      defaultCognitoRegion,
			UserPoolID:  defaultCognitoUserPoolID,
			AppClientID: defaultCognitoAppClientID,
		},
		Redis: RedisConfig{
			Addr:     defaultRedisAddr,
			Password: defaultRedisPassword,
			DB:       defaultRedisDB,
		},
	}
}

func NewConfig(options ...func(*Config)) Config {
	config := DefaultConfig()
	for _, opt := range options {
		opt(&config)
	}
	return config
}

func NewConfigFromEnv(options ...func(*Config)) (Config, error) {
	config := DefaultConfig()
	err := errors.Join(
		setFromEnv(&config.Environment, "ENVIRONMENT"),
		setFromEnv(&config.Port, "PORT"),
		setFromEnv(&config.SkipAuth, "SKIP_AUTH"),
		setFromEnv(&config.Otel.Disable, "OTEL_DISABLE"),
		setFromEnv(&config.Otel.OtlpExporter.Endpoint, "OTEL_OTLP_EXPORTER_ENDPOINT"),
		setFromEnv(&config.Otel.OtlpExporter.Insecure, "OTEL_OTLP_EXPORTER_INSECURE"),
		setFromEnv(&config.Cognito.Region, "COGNITO_REGION"),
		setFromEnv(&config.Cognito.UserPoolID, "COGNITO_USER_POOL_ID"),
		setFromEnv(&config.Cognito.AppClientID, "COGNITO_APP_CLIENT_ID"),
		setFromEnv(&config.Redis.Addr, "REDIS_ADDR"),
		setFromEnv(&config.Redis.Password, "REDIS_PASSWORD"),
		setFromEnv(&config.Redis.DB, "REDIS_DB"),
	)

	for _, opt := range options {
		opt(&config)
	}

	return config, err
}

func LoadEnv(environment ...string) error {
	filenames := []string{
		".env.local",
		".env",
	}

	env := getEnv("ENVIRONMENT", DefaultConfig().Environment)
	if len(environment) > 0 {
		env = environment[0]
	}

	if env != "" {
		file := ".env." + env + ".local"
		filenames = append([]string{file}, filenames...)
	}

	var errs error

	for _, filename := range filenames {
		err := loadEnvFile(filename)
		if err != nil {
			errs = errors.Join(
				errs,
				fmt.Errorf("error loading %s: %w", filename, err),
			)
		}
	}

	return errs
}
