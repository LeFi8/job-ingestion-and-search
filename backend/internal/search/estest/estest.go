//go:build integration

package estest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	httpPort       = "9200/tcp"
	startupTimeout = 3 * time.Minute
)

func Start(ctx context.Context) (string, func(), error) {
	container, err := testcontainers.Run(ctx,
		"docker.elastic.co/elasticsearch/elasticsearch:9.5.4",
		testcontainers.WithExposedPorts(httpPort),
		testcontainers.WithEnv(map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
			"cluster.routing.allocation.disk.threshold_enabled": "false",
			"ingest.geoip.downloader.enabled":                   "false",
			"ES_JAVA_OPTS":                                      "-Xms512m -Xmx512m",
		}),
		testcontainers.WithWaitStrategyAndDeadline(startupTimeout,
			wait.ForHTTP("/").WithPort(httpPort).WithStartupTimeout(startupTimeout),
		),
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
	url, err := container.PortEndpoint(ctx, httpPort, "http")
	if err != nil {
		stop()
		return "", nil, fmt.Errorf("elasticsearch endpoint: %w", err)
	}
	return url, stop, nil
}
