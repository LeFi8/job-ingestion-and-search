package config

import "time"

const MaxWorkerBatchSize = 10000

type Migrate struct {
	DatabaseURL string
}

type Ingest struct {
	DatabaseURL string
}

type Worker struct {
	DatabaseURL      string
	ElasticsearchURL string
	BatchSize        int
	PollInterval     time.Duration
}

func LoadMigrate() (Migrate, error) {
	var env Env
	cfg := Migrate{DatabaseURL: env.String("DATABASE_URL")}
	return cfg, env.Err()
}

func LoadIngest() (Ingest, error) {
	var env Env
	cfg := Ingest{DatabaseURL: env.String("DATABASE_URL")}
	return cfg, env.Err()
}

func LoadWorker() (Worker, error) {
	var env Env
	cfg := Worker{
		DatabaseURL:      env.String("DATABASE_URL"),
		ElasticsearchURL: env.String("ELASTICSEARCH_URL"),
		BatchSize:        env.IntBetween("WORKER_BATCH_SIZE", 1, MaxWorkerBatchSize),
		PollInterval:     env.PositiveDuration("WORKER_POLL_INTERVAL"),
	}
	return cfg, env.Err()
}
