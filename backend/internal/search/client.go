package search

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

const (
	indexName = "jobs_v1"
	aliasName = "jobs"
)

//go:embed mapping.json
var mappingJSON []byte

type Client struct {
	es *elasticsearch.TypedClient
}

func New(url string) (*Client, error) {
	es, err := elasticsearch.NewTyped(elasticsearch.WithAddresses(url))
	if err != nil {
		return nil, fmt.Errorf("new elasticsearch client: %w", err)
	}
	return &Client{es: es}, nil
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	req, err := createIndexRequest()
	if err != nil {
		return err
	}
	_, err = c.es.Indices.Create(indexName).Request(req).Do(ctx)
	if err == nil || isAlreadyExists(err) {
		return nil
	}
	return fmt.Errorf("create index %s: %w", indexName, err)
}

func createIndexRequest() (*create.Request, error) {
	req, err := create.NewRequest().FromJSON(string(mappingJSON))
	if err != nil {
		return nil, fmt.Errorf("decode embedded mapping: %w", err)
	}
	req.Aliases = map[string]types.Alias{aliasName: {}}
	return req, nil
}

func isAlreadyExists(err error) bool {
	var esErr *types.ElasticsearchError
	return errors.As(err, &esErr) && esErr.ErrorCause.Type == "resource_already_exists_exception"
}
