package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vivlilv/eneba_task/api/internal/app"
	"github.com/vivlilv/eneba_task/api/internal/migrations"
)

func main() {
	databaseURL := env("DATABASE_URL", "postgres://eneba:eneba_dev@localhost:5432/eneba?sslmode=disable")
	addr := env("API_ADDR", ":8080")
	cookieSecure := env("SESSION_COOKIE_SECURE", "false") == "true"

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := migrations.Up(ctx, db); err != nil {
		log.Fatalf("apply database migrations: %v", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           app.New(db, cookieSecure).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("API listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
