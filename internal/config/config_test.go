package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnv(t *testing.T) {
	err := loadEnv(".env.example")
	// if err != nil {
	// 	t.Fatalf(`loadEnv(".env.example" returned error: %v)`, err)
	// }

	require.NoErrorf(t, err, `loadEnv(".env.example" returned error: %v)`, err)
}

func TestGetEnv_KeyValue(t *testing.T) {
	t.Setenv("xyz", "abc")

	result := getEnv("xyz", "development")

	expected := "abc"

	// if result != expected {
	// 	t.Errorf(`getEnv("xyz", "development") = %q; expected: %q`, result, expected)
	// }
	assert.Equalf(t, expected, result, `getEnv("xyz", "development") = %q; expected: %q`, result, expected)
}

func TestGetEnv_FallbackValue(t *testing.T) {
	// set test env var to empty to trigger fallback
	t.Setenv("xyz", "")

	result := getEnv("xyz", "development")

	expected := "development"

	assert.Equalf(t, expected, result, `getEnv("xyz", "development") = %q; expected: %q`, result, expected)
}
