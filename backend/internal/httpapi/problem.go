package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-gonic/gin"
)

func writeProblem(c *gin.Context, status int, detail string) {
	p := Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, p)
}

func recoverWithProblem(c *gin.Context, recovered any) {
	_ = c.Error(fmt.Errorf("panic: %v\n%s", recovered, debug.Stack()))
	writeProblem(c, http.StatusInternalServerError, "")
}

func describeValidationError(err error) string {
	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) || reqErr.Parameter == nil {
		return "The request does not match the API contract."
	}
	reason := reqErr.Reason
	var schemaErr *openapi3.SchemaError
	if errors.As(reqErr.Err, &schemaErr) && schemaErr.Reason != "" {
		reason = schemaErr.Reason
	}
	if reason == "" {
		reason = reqErr.Err.Error()
	}
	return fmt.Sprintf("parameter %q: %s", reqErr.Parameter.Name, reason)
}
