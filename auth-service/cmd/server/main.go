package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"auth-service/internal/platform/db"
	"auth-service/internal/user"
)

type response struct {
	Service     string `json:"service"`
	Status      string `json:"status"`
	DatabaseURL string `json:"database_url"`
	RedisAddr   string `json:"redis_addr"`
}

func main() {
	serviceName := getenv("SERVICE_NAME", "auth-service")
	port := getenv("PORT", "8080")
	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "dev-secret-change-me")
	jwtIssuer := getenv("JWT_ISSUER", "auth-service")
	jwtTTL := 24 * time.Hour

	pool, err := db.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	repo := user.NewPGRepository(pool)
	svc, err := user.NewService(repo, jwtSecret, jwtIssuer, jwtTTL)
	if err != nil {
		log.Fatalf("failed to init auth service: %v", err)
	}
	userHandler := user.NewHandler(svc)

	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{
			Service:     serviceName,
			Status:      "ok",
			DatabaseURL: databaseURL,
			RedisAddr:   getenv("REDIS_ADDR", "not-set"),
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Welcome to auth-service",
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
