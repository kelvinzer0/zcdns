package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zcdns-backend/api"
	"zcdns-backend/config"
	"zcdns-backend/db"
	"zcdns-backend/dns"
	"zcdns-backend/parental"
)

func main() {
	cfg := config.LoadConfig()
	log.Printf("==================================================")
	log.Printf("    ZeroCentDNS (ZCDNS) Backend Engine")
	log.Printf("    Base Domain : *.%s", cfg.BaseDomain)
	log.Printf("    HTTP Port   : %s", cfg.HTTPPort)
	log.Printf("    DNS Port    : %d (UDP/TCP)", cfg.DNSPort)
	log.Printf("    DoT Port    : %d (TCP-TLS)", cfg.DoTPort)
	log.Printf("    SQLite DB   : %s", cfg.DBPath)
	log.Printf("==================================================")

	database, err := db.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Initialize WebSocket stream hub
	hub := api.NewStreamHub()
	go hub.Run()

	// Initialize Parental Control & Web Blocker Engine
	parentalEngine := parental.NewEngine(cfg, database, hub)

	// Initialize and start Authoritative DNS Server
	dnsServer := dns.NewServer(cfg, database, hub, parentalEngine)
	if err := dnsServer.Start(); err != nil {
		log.Printf("[WARN] Failed to start DNS server on port %d: %v", cfg.DNSPort, err)
	} else {
		log.Printf("[DNS] DNS Server listening on %v", cfg.DNSAddrs)
	}
	if err := dnsServer.StartDoT(); err != nil {
		log.Printf("[WARN] Failed to start DoT server on port %d: %v", cfg.DoTPort, err)
	}
	defer dnsServer.Stop()

	// Initialize HTTP API
	apiHandler := api.NewAPIHandler(cfg, database, hub, parentalEngine)
	apiHandler.SetOnRecordChanged(dnsServer.IncrementSerialAndNotify)
	mux := http.NewServeMux()
	apiHandler.RegisterRoutes(mux)

	// Health check endpoint
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","baseDomain":"%s","dnsPort":%d}`, cfg.BaseDomain, cfg.DNSPort)
	}
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/health", healthHandler)

	// Periodic cleanup worker (runs every CleanInterval minutes)
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.CleanInterval) * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			purged := database.CleanupOldData()
			if len(purged) > 0 {
				dnsServer.IncrementSerialAndNotify()
			}
		}
	}()

	server := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("[HTTP] API Server listening on %s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	signal.Ignore(syscall.SIGHUP)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[INFO] Shutting down ZCDNS server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server shutdown error: %v", err)
	}

	log.Println("[INFO] Server stopped gracefully.")
}
