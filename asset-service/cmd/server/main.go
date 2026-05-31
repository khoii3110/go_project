package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"asset-service/internal/asset"
	"asset-service/internal/platform/cache"
	"asset-service/internal/platform/db"
	"asset-service/internal/platform/messaging"
)

type response struct {
	Service     string `json:"service"`
	Status      string `json:"status"`
	DatabaseURL string `json:"database_url"`
	RedisAddr   string `json:"redis_addr"`
}

func main() {
	serviceName := getenv("SERVICE_NAME", "asset-service")
	port := getenv("PORT", "8080")
	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/asset_db?sslmode=disable")
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	metaPrefix := getenv("REDIS_ASSET_META_PREFIX", "asset:{assetId}:meta")
	aclPrefix := getenv("REDIS_ASSET_ACL_PREFIX", "asset:{assetId}:acl")
	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	rabbitExchange := getenv("RABBITMQ_ASSET_EXCHANGE", "asset.changes")

	pool, err := db.Connect(context.Background(), databaseURL)
	if err != nil { log.Fatalf("failed to connect database: %v", err) }
	defer pool.Close()

	assetCache := cache.NewRedisCache(redisAddr, metaPrefix, aclPrefix, 10*time.Minute)
	defer assetCache.Close()

	publisher, err := messaging.NewPublisher(rabbitURL, rabbitExchange)
	if err != nil { log.Fatalf("failed to connect rabbitmq: %v", err) }
	defer publisher.Close()

	repo := asset.NewPGRepository(pool)
	svc := asset.NewService(repo, assetCache, publisher)
	handler := asset.NewHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{
			Service:     serviceName,
			Status:      "ok",
			DatabaseURL: databaseURL,
			RedisAddr:   redisAddr,
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Welcome to asset-service",
		})
	})

	log.Printf("%s listening on :%s", serviceName, port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
