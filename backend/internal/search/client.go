package search

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
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
	body, err := createIndexBody()
	if err != nil {
		return err
	}
	_, err = c.es.Indices.Create(indexName).Raw(bytes.NewReader(body)).Do(ctx)
	if err == nil || isAlreadyExists(err) {
		return nil
	}
	return fmt.Errorf("create index %s: %w", indexName, err)
}

func createIndexBody() ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(mappingJSON, &body); err != nil {
		return nil, fmt.Errorf("decode embedded mapping: %w", err)
	}
	body["aliases"] = json.RawMessage(`{"` + aliasName + `":{}}`)
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode index body: %w", err)
	}
	return out, nil
}

func isAlreadyExists(err error) bool {
	var esErr *types.ElasticsearchError
	return errors.As(err, &esErr) && esErr.ErrorCause.Type == "resource_already_exists_exception"
}
