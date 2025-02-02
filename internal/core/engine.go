package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labs-alone/alone-main/internal/core"
	"github.com/labs-alone/alone-main/internal/openai"
	"github.com/labs-alone/alone-main/internal/solana"
	"github.com/labs-alone/alone-main/internal/utils"
)

func main() {
	// Initialize logger
	logger := utils.NewLogger()
	logger.Info("Starting Alone Labs CLI...")

	// Load configuration
	config, err := utils.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load configuration:", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize core engine
	engine, err := core.NewEngine(config)
	if err != nil {
		logger.Fatal("Failed to initialize engine:", err)
	}

	// Initialize Solana client
	solanaClient, err := solana.NewClient(config.Solana)
	if err != nil {
		logger.Fatal("Failed to initialize Solana client:", err)
	}

	// Initialize OpenAI client
	openaiClient, err := openai.NewClient(config.OpenAI)
	if err != nil {
		logger.Fatal("Failed to initialize OpenAI client:", err)
	}

	// Print startup banner
	printBanner()

	// Start the engine
	go func() {
		if err := engine.Start(ctx); err != nil {
			logger.Error("Engine error:", err)
			cancel()
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for interrupt signal
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal")
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := engine.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during shutdown:", err)
	}

	logger.Info("Shutdown complete")
}

func printBanner() {
	banner := `
    _    _                  _           _         
   / \  | | ___  _ __   __| |    _    | |    ___ 
  / _ \ | |/ _ \| '_ \ / _' |  _| |_  | |   / _ \
 / ___ \| | (_) | | | | (_| | |_   _| | |__|  __/
/_/   \_\_|\___/|_| |_|\__,_|   |_|   |_____\___|

Alone Labs CLI - Version 0.1.0
Blockchain Integration & AI Processing Engine
`
	fmt.Println(banner)
}

func init() {
	// Set up any necessary environment initialization
	if err := utils.InitializeEnvironment(); err != nil {
		log.Fatal("Failed to initialize environment:", err)
	}
}