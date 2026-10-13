package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Env struct{ errs []error }

func (e *Env) String(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		e.errs = append(e.errs, fmt.Errorf("%s: missing", key))
		return ""
	}
	return value
}

func (e *Env) Int(key string) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		e.errs = append(e.errs, fmt.Errorf("%s: missing", key))
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
	raw, ok := os.LookupEnv(key)
	if !ok {
		e.errs = append(e.errs, fmt.Errorf("%s: missing", key))
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
