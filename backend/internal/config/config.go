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

func (e *Env) Err() error {
	return errors.Join(e.errs...)
}
