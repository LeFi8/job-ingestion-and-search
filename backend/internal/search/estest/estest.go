//go:build integration

package estest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/testcontainers/testcontainers-go"
	tcelasticsearch "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
)

func Start(ctx context.Context) (string, func(), error) {
	container, err := tcelasticsearch.Run(ctx,
		"docker.elastic.co/elasticsearch/elasticsearch:9.5.4",
		testcontainers.WithEnv(map[string]string{"xpack.security.enabled": "false"}),
	)
	if err != nil {
		if container != nil {
			_ = testcontainers.TerminateContainer(container)
		}
		return "", nil, fmt.Errorf("start elasticsearch container: %w", err)
	}
	stop := func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			slog.Error("terminate elasticsearch container", "error", err)
		}
	}
	return container.Settings.Address, stop, nil
}
