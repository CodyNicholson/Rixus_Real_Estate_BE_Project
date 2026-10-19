package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
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

	var cfValidator *auth.CFValidator
	if cfg.CFTeamDomain != "" || cfg.CFPolicyAUD != "" || cfg.CFCertsURL != "" {
		cfValidator = auth.NewCFValidator(cfg.CFTeamDomain, cfg.CFPolicyAUD, cfg.CFCertsURL)
		log.Printf("cloudflare_access: JWT assertion verification enabled (team=%q, aud=%q)", cfg.CFTeamDomain, cfg.CFPolicyAUD)
	}

	s := &handlers.Server{
		DB:           pool,
		JWTSecret:    cfg.JWTSecret,
		CORSOrigin:   cfg.CORSOrigin,
		CFValidator:  cfValidator,
		DevCFEmail:   cfg.DevCFEmail,
		AllowDevAuth: cfg.AllowDevAuth,
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           s.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
