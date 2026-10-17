package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Env struct{ errs []error }

func (e *Env) lookup(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		e.errs = append(e.errs, fmt.Errorf("%s: missing", key))
		return "", false
	}
	if strings.TrimSpace(value) == "" {
		e.errs = append(e.errs, fmt.Errorf("%s: empty", key))
		return "", false
	}
	return value, true
}

func (e *Env) String(key string) string {
	value, _ := e.lookup(key)
	return value
}

func (e *Env) Int(key string) int {
	raw, ok := e.lookup(key)
	if !ok {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s: %q is not an integer", key, raw))
		return 0
	}
	return value
}

func (e *Env) IntBetween(key string, lo, hi int) int {
	before := len(e.errs)
	value := e.Int(key)
	if len(e.errs) > before {
		return 0
	}
	if value < lo || value > hi {
		e.errs = append(e.errs, fmt.Errorf("%s: %d is outside %d..%d", key, value, lo, hi))
		return 0
	}
	return value
}

func (e *Env) PositiveDuration(key string) time.Duration {
	before := len(e.errs)
	value := e.Duration(key)
	if len(e.errs) > before {
		return 0
	}
	if value <= 0 {
		e.errs = append(e.errs, fmt.Errorf("%s: %s is not positive", key, value))
		return 0
	}
	return value
}

func (e *Env) Duration(key string) time.Duration {
	raw, ok := e.lookup(key)
	if !ok {
		return 0
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s: %q is not a duration: %w", key, raw, err))
		return 0
	}
	return value
}

func (e *Env) DatabaseURL() string {
	return e.String("DATABASE_URL")
}

func (e *Env) Err() error {
	return errors.Join(e.errs...)
}
