package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Configurable grace period (3 seconds default)
	gracePeriod := 3 * time.Second

	// Create server with grace period
	server := NewServer(gracePeriod)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverDone:
		if err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	case sig := <-shutdown:
		fmt.Printf("Received signal: %v\n", sig)
		fmt.Printf("Initiating graceful shutdown with %v grace period...\n", gracePeriod)

		// Cancel context to trigger graceful shutdown
		cancel()

		// Wait for server to finish graceful shutdown
		err := <-serverDone
		if err != nil {
			fmt.Printf("Server shutdown error: %v\n", err)
		} else {
			fmt.Println("Server shut down gracefully")
		}
	}
}
