package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type response struct {
	Service     string `json:"service"`
	Status      string `json:"status"`
	DatabaseURL string `json:"database_url"`
	RedisAddr   string `json:"redis_addr"`
}

func main() {
	serviceName := getenv("SERVICE_NAME", "team-service")
	port := getenv("PORT", "8080")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{
			Service:     serviceName,
			Status:      "ok",
			DatabaseURL: getenv("DATABASE_URL", "not-set"),
			RedisAddr:   getenv("REDIS_ADDR", "not-set"),
		})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Welcome to team-service",
		})
	})

	log.Printf("%s listening on :%s", serviceName, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
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
