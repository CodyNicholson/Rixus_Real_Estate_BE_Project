package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/config"
	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/db"
	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/handlers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	s := &handlers.Server{DB: pool, JWTSecret: cfg.JWTSecret, CORSOrigin: cfg.CORSOrigin}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           s.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
