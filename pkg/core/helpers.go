package core

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func setFromEnv(loc any, key string) error {
	strValue := os.Getenv(key)
	if strValue == "" {
		return nil
	}

	switch v := loc.(type) {
	case *string:
		*v = strValue
	case *bool:
		val, err := strconv.ParseBool(strValue)
		if err != nil {
			return fmt.Errorf("failed to parse %s as a bool: %w", strValue, err)
		}
		*v = val
	case *int:
		val, err := strconv.ParseInt(strValue, 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("failed to parse %s as an int: %w", strValue, err)
		}
		*v = int(val)
	}
	return nil
}

func loadEnvFile(filename string) error {
	err := godotenv.Load(filename)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("error loading file %s: %w", filename, err)
}

func getEnv(key, fallback string) string {
	// returns value of associated env key
	value := os.Getenv(key)

	if value != "" {
		return value
	}

	return fallback
}

func (c *Config) IsProd() bool {
	if c == nil || c.Environment != "production" {
		return false
	}

	return true
}
