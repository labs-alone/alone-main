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

	"github.com/alone-labs/internal/api"
	"github.com/alone-labs/internal/config"
	"github.com/alone-labs/internal/database"
	"github.com/alone-labs/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()
	log.Info("Starting Alone Labs server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database connection
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize router and API handlers
	router := api.NewRouter(db, log)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info(fmt.Sprintf("Server starting on port %d", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Info("Server exited properly")
}