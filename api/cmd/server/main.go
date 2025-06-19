package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/server"
)

func main() {
	config, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer pool.Close()

	store := db.NewStore(pool)

	redisAddr := fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: config.RedisUsername,
		Password: config.RedisPassword,
	})
	defer asynqClient.Close()

	api, err := server.NewServer(config, store, asynqClient)
	if err != nil {
		log.Fatalf("cannot create server: %v", err)
	}

	if err := api.Start(config.ServerAddress); err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
}
