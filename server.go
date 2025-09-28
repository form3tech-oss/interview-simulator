package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Server defines TCP server with graceful shutdown capabilities
type Server struct {
	listener    net.Listener
	connections map[net.Conn]bool
	mu          sync.RWMutex
	gracePeriod time.Duration
}

// NewServer creates a new server instance with the specified grace period
func NewServer(gracePeriod time.Duration) *Server {
	return &Server{
		connections: make(map[net.Conn]bool),
		gracePeriod: gracePeriod,
	}
}

// Start starts the server and listens for incoming connections
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", 8080))
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()

	// Channel to signal when we should stop accepting new connections
	acceptDone := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutine to handle shutdown
	go func() {
		<-ctx.Done()
		fmt.Println("Shutdown signal received, stopping acceptance of new connections...")

		// Close listener to stop accepting new connections
		listener.Close()

		// Wait for grace period, then force close remaining connections
		graceTimer := time.NewTimer(s.gracePeriod)
		defer graceTimer.Stop()

		select {
		case <-graceTimer.C:
			fmt.Printf("Grace period of %v expired, closing remaining connections...\n", s.gracePeriod)
			s.closeAllConnections()
		case <-acceptDone:
			fmt.Println("All connections closed gracefully")
		}
	}()

	// Accept connections until listener is closed
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if this is due to listener being closed (shutdown)
			select {
			case <-ctx.Done():
				fmt.Println("Stopped accepting new connections")
				break
			default:
				fmt.Println("Error accepting connection:", err)
				continue
			}
			break
		}

		// Track the new connection
		s.addConnection(conn)
		wg.Add(1)

		go func(conn net.Conn) {
			defer wg.Done()
			s.handleConnection(ctx, conn)
		}(conn)
	}

	// Wait for all connections to finish
	wg.Wait()
	close(acceptDone)
	return nil
}

// addConnection adds a connection to the tracking map
func (s *Server) addConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[conn] = true
}

// removeConnection removes a connection from the tracking map
func (s *Server) removeConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, conn)
}

// closeAllConnections forcefully closes all tracked connections
func (s *Server) closeAllConnections() {
	s.mu.RLock()
	conns := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		conns = append(conns, conn)
	}
	s.mu.RUnlock()

	for _, conn := range conns {
		conn.Close()
	}
}

// handleConnection handles individual client connections
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		s.removeConnection(conn)
	}()

	// Track if shutdown has been initiated
	shutdownStarted := false
	shutdownTime := time.Time{}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		request := scanner.Text()

		// Check if shutdown was initiated while we were waiting for requests
		select {
		case <-ctx.Done():
			if !shutdownStarted {
				shutdownStarted = true
				shutdownTime = time.Now()
			}
		default:
		}

		// If shutdown started, check if grace period has expired
		if shutdownStarted && time.Since(shutdownTime) > s.gracePeriod {
			fmt.Fprintf(conn, "RESPONSE|REJECTED|Cancelled\n")
			return
		}

		// Process the request
		requestDone := make(chan string, 1)

		go func(req string) {
			// Create a background context for request processing
			// Don't use the main ctx as it gets cancelled immediately on shutdown
			requestCtx := context.Background()
			response := s.handleRequest(requestCtx, req)
			requestDone <- response
		}(request)

		// Wait for request to complete, but respect grace period if shutdown has started
		var response string
		if shutdownStarted {
			// During shutdown, enforce grace period
			remainingGrace := s.gracePeriod - time.Since(shutdownTime)
			if remainingGrace <= 0 {
				response = "RESPONSE|REJECTED|Cancelled"
			} else {
				select {
				case response = <-requestDone:
					// Request completed within grace period
				case <-time.After(remainingGrace):
					response = "RESPONSE|REJECTED|Cancelled"
				}
			}
		} else {
			// Normal operation
			select {
			case response = <-requestDone:
				// Request completed normally
			case <-ctx.Done():
				// Shutdown initiated during request processing
				shutdownStarted = true
				shutdownTime = time.Now()

				// Allow current request to complete within grace period
				select {
				case response = <-requestDone:
					// Request completed within grace period
				case <-time.After(s.gracePeriod):
					response = "RESPONSE|REJECTED|Cancelled"
				}
			}
		}

		fmt.Fprintf(conn, "%s\n", response)

		// If we sent a cancellation response, close connection
		if response == "RESPONSE|REJECTED|Cancelled" {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from connection:", err)
	}
}
