package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"team-service/internal/platform/auth"
	redisCache "team-service/internal/platform/cache"
	"team-service/internal/platform/db"
	"team-service/internal/platform/messaging"
	"team-service/internal/team"
)

type response struct {
	Service     string `json:"service"`
	Status      string `json:"status"`
	DatabaseURL string `json:"database_url"`
}

func main() {
	serviceName := getenv("SERVICE_NAME", "team-service")
	port := getenv("PORT", "8080")
	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/team_db?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "dev-secret-change-me")
	jwtIssuer := getenv("JWT_ISSUER", "auth-service")
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	memberCachePrefix := getenv("REDIS_TEAM_MEMBER_LIST_PREFIX", "team:members")
	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	rabbitExchange := getenv("RABBITMQ_TEAM_EXCHANGE", "team.activity")

	pool, err := db.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	cache := redisCache.NewRedisCache(redisAddr, memberCachePrefix, 5*time.Minute)
	defer cache.Close()

	publisher, err := messaging.NewPublisher(rabbitURL, rabbitExchange)
	if err != nil {
		log.Fatalf("failed to connect rabbitmq: %v", err)
	}
	defer publisher.Close()

	repo := team.NewPGRepository(pool)
	svc := team.NewService(repo, cache, publisher)
	handler := team.NewHandler(svc)
	authMiddleware := auth.NewMiddleware(jwtSecret, jwtIssuer)

	go func() {
		if err := messaging.StartTeamCacheInvalidator(context.Background(), rabbitURL, rabbitExchange, "team.cache.invalidations", cache); err != nil {
			log.Printf("team cache invalidator stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{Service: serviceName, Status: "ok", DatabaseURL: databaseURL})
	})

	protected := authMiddleware.RequireAuth(mux)

	log.Printf("%s listening on :%s", serviceName, port)
	if err := http.ListenAndServe(":"+port, protected); err != nil {
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
