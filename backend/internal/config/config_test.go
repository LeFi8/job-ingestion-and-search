package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

const (
	keyA = "JIS_TEST_CFG_A"
	keyB = "JIS_TEST_CFG_B"
	keyC = "JIS_TEST_CFG_C"
)

func unsetForTest(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%s): %v", key, err)
		}
	}
}

func TestEnvReportsAllMissingKeys(t *testing.T) {
	unsetForTest(t, keyA, keyB, keyC)

	var env Env
	env.String(keyA)
	env.Int(keyB)
	env.Duration(keyC)

	err := env.Err()
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
	for _, key := range []string{keyA, keyB, keyC} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("Err() = %q, want it to mention %s", err, key)
		}
	}
}

func TestEnvIntRejectsNonInteger(t *testing.T) {
	t.Setenv(keyB, "abc")

	var env Env
	env.Int(keyB)

	err := env.Err()
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
	for _, want := range []string{keyB, "not an integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Err() = %q, want it to contain %q", err, want)
		}
	}
}

func TestEnvDurationRejectsInvalid(t *testing.T) {
	t.Setenv(keyC, "soon")

	var env Env
	env.Duration(keyC)

	err := env.Err()
	if err == nil {
		t.Fatal("Err() = nil, want error")
	}
	if !strings.Contains(err.Error(), keyC) {
		t.Errorf("Err() = %q, want it to mention %s", err, keyC)
	}
}

func TestEnvReadsValidValues(t *testing.T) {
	t.Setenv(keyA, "hello")
	t.Setenv(keyB, "42")
	t.Setenv(keyC, "1m30s")

	var env Env
	if got := env.String(keyA); got != "hello" {
		t.Errorf("String = %q, want hello", got)
	}
	if got := env.Int(keyB); got != 42 {
		t.Errorf("Int = %d, want 42", got)
	}
	if got := env.Duration(keyC); got != 90*time.Second {
		t.Errorf("Duration = %v, want 1m30s", got)
	}
	if err := env.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestEnvRejectsEmptyValues(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Setenv(keyA, value)
		t.Setenv(keyB, value)
		t.Setenv(keyC, value)

		var env Env
		env.String(keyA)
		env.Int(keyB)
		env.Duration(keyC)

		err := env.Err()
		if err == nil {
			t.Fatalf("value %q: Err() = nil, want error", value)
		}
		for _, key := range []string{keyA, keyB, keyC} {
			if want := key + ": empty"; !strings.Contains(err.Error(), want) {
				t.Errorf("value %q: Err() = %q, want it to contain %q", value, err, want)
			}
		}
	}
}
