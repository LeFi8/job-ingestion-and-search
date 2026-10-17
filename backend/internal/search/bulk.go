package search

import (
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/versiontype"
)

type Doc struct {
	JobID            string `json:"job_id"`
	Title            string `json:"title"`
	Company          string `json:"company"`
	City             string `json:"city,omitempty"`
	Country          string `json:"country,omitempty"`
	Remote           bool   `json:"remote"`
	SalaryUSDAnnual  int64  `json:"salary_usd_annual"`
	SalaryDisplay    string `json:"salary_display"`
	CurrencyInferred bool   `json:"currency_inferred"`
	PostedOn         string `json:"posted_on,omitempty"`
}

type Action struct {
	Version int64
	JobID   string
	Doc     *Doc
}

type ItemResult struct {
	Version int64
	Err     error
}

func (c *Client) Bulk(ctx context.Context, actions []Action) ([]ItemResult, error) {
	if len(actions) == 0 {
		return nil, nil
	}

	req := c.es.Bulk().Index(aliasName).RequireAlias(true)
	for _, a := range actions {
		if err := addAction(req, a); err != nil {
			return nil, err
		}
	}
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("bulk request: %w", err)
	}
	if len(resp.Items) != len(actions) {
		return nil, fmt.Errorf("bulk response has %d items for %d actions", len(resp.Items), len(actions))
	}

	results := make([]ItemResult, len(actions))
	for i, item := range resp.Items {
		if len(item) != 1 {
			return nil, fmt.Errorf("bulk response item %d has %d operations", i, len(item))
		}
		for _, r := range item {
			if r.Status == http.StatusTooManyRequests {
				return nil, fmt.Errorf("bulk item %d: elasticsearch overloaded (429)", i)
			}
			if r.Status >= http.StatusInternalServerError {
				return nil, fmt.Errorf("bulk item %d: elasticsearch unavailable (%d)", i, r.Status)
			}
			if r.Error != nil && r.Error.Type == "index_not_found_exception" {
				return nil, fmt.Errorf("bulk item %d: alias %s is missing: %s", i, aliasName, r.Error.Type)
			}
			results[i] = ItemResult{Version: actions[i].Version, Err: itemError(actions[i], r)}
		}
	}
	return results, nil
}

type bulkOps interface {
	IndexOp(op types.IndexOperation, doc any) error
	DeleteOp(op types.DeleteOperation) error
}

func addAction(req bulkOps, a Action) error {
	id := a.JobID
	version := a.Version
	external := versiontype.External
	if a.Doc == nil {
		return req.DeleteOp(types.DeleteOperation{Id_: &id, Version: &version, VersionType: &external})
	}
	if a.Doc.JobID != a.JobID {
		return fmt.Errorf("action JobID %q does not match Doc.JobID %q", a.JobID, a.Doc.JobID)
	}
	return req.IndexOp(types.IndexOperation{Id_: &id, Version: &version, VersionType: &external}, a.Doc)
}

func itemError(a Action, r types.ResponseItem) error {
	switch {
	case r.Status < 300:
		return nil
	case r.Status == http.StatusConflict:
		return nil
	case r.Status == http.StatusNotFound && a.Doc == nil:
		return nil
	}
	if r.Error == nil {
		return fmt.Errorf("bulk item %s: status %d", a.JobID, r.Status)
	}
	reason := ""
	if r.Error.Reason != nil {
		reason = *r.Error.Reason
	}
	return fmt.Errorf("bulk item %s: status %d: %s: %s", a.JobID, r.Status, r.Error.Type, reason)
}
