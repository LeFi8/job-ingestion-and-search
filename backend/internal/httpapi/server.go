package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
)

func NewRouter(s JobSearcher, searchTimeout time.Duration, log *slog.Logger) (*gin.Engine, error) {
	spec, err := GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	r.Use(accessLog(log), gin.CustomRecoveryWithWriter(io.Discard, recoverWithProblem))
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	validator, err := validateRequest(spec)
	if err != nil {
		return nil, err
	}
	api := r.Group("", validator)
	RegisterHandlersWithOptions(api, handler{searcher: s, searchTimeout: searchTimeout}, GinServerOptions{
		ErrorHandler: func(c *gin.Context, err error, status int) { writeProblem(c, status, err.Error()) },
	})
	return r, nil
}

func validateRequest(spec *openapi3.T) (gin.HandlerFunc, error) {
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return nil, fmt.Errorf("build openapi router: %w", err)
	}
	return func(c *gin.Context) {
		route, pathParams, err := router.FindRoute(c.Request)
		if err != nil {
			_ = c.Error(fmt.Errorf("gin route missing from openapi spec: %w", err))
			writeProblem(c, http.StatusInternalServerError, "")
			return
		}
		err = openapi3filter.ValidateRequest(c.Request.Context(), &openapi3filter.RequestValidationInput{
			Request:    c.Request,
			PathParams: pathParams,
			Route:      route,
			Options:    &openapi3filter.Options{SkipSettingDefaults: true},
		})
		if err != nil {
			writeProblem(c, http.StatusBadRequest, describeValidationError(err))
		}
	}, nil
}

func accessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"status", status,
			"duration", time.Since(start),
		}
		level := slog.LevelInfo
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.String())
		}
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		log.Log(c.Request.Context(), level, "http.request", attrs...)
	}
}
