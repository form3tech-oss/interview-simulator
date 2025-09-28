package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to start a test server
func startTestServer(gracePeriod time.Duration) (*Server, context.CancelFunc, error) {
	server := NewServer(gracePeriod)
	ctx, cancel := context.WithCancel(context.Background())

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(ctx)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Test if server is accepting connections
	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		cancel()
		return nil, nil, err
	}
	conn.Close()

	return server, cancel, nil
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestBasicFunctionality(t *testing.T) {
	// Start test server
	server, cancel, err := startTestServer(3 * time.Second)
	require.NoError(t, err, "Failed to start test server")
	defer cancel()
	_ = server // Use server variable to avoid linter warning

	tests := []struct {
		name           string
		input          string
		expectedOutput string
		minDuration    time.Duration
		maxDuration    time.Duration
	}{
		{
			name:           "Valid Request",
			input:          "PAYMENT|10",
			expectedOutput: "RESPONSE|ACCEPTED|Transaction processed",
			maxDuration:    50 * time.Millisecond,
		},
		{
			name:           "Valid Request with Delay",
			input:          "PAYMENT|101",
			expectedOutput: "RESPONSE|ACCEPTED|Transaction processed",
			minDuration:    101 * time.Millisecond,
			maxDuration:    200 * time.Millisecond,
		},
		{
			name:           "Invalid Request Format",
			input:          "INVALID|100",
			expectedOutput: "RESPONSE|REJECTED|Invalid request",
			maxDuration:    10 * time.Millisecond,
		},
		{
			name:           "Invalid Amount",
			input:          "PAYMENT|abc",
			expectedOutput: "RESPONSE|REJECTED|Invalid amount",
			maxDuration:    10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ":8080")
			require.NoError(t, err, "Failed to connect to server")
			defer conn.Close()

			_, err = fmt.Fprintf(conn, tt.input+"\n")
			require.NoError(t, err, "Failed to send request")

			start := time.Now()

			response, err := bufio.NewReader(conn).ReadString('\n')
			require.NoError(t, err, "Failed to read response")
			duration := time.Since(start)

			response = strings.TrimSpace(response)

			assert.Equal(t, tt.expectedOutput, response, "Unexpected response")

			if tt.minDuration > 0 {
				assert.GreaterOrEqual(t, duration, tt.minDuration, "Response time was shorter than expected")
			}

			if tt.maxDuration > 0 {
				assert.LessOrEqual(t, duration, tt.maxDuration, "Response time was longer than expected")
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	gracePeriod := 1 * time.Second

	// Start test server
	_, cancel, err := startTestServer(gracePeriod)
	require.NoError(t, err, "Failed to start test server")
	defer cancel()

	// Establish multiple connections
	conn1, err := net.Dial("tcp", ":8080")
	require.NoError(t, err, "Failed to connect to server")
	defer conn1.Close()

	conn2, err := net.Dial("tcp", ":8080")
	require.NoError(t, err, "Failed to connect to server")
	defer conn2.Close()

	// Start a long-running request on conn1
	longRunningDone := make(chan string, 1)
	go func() {
		fmt.Fprintf(conn1, "PAYMENT|500\n") // 500ms processing time
		response, err := bufio.NewReader(conn1).ReadString('\n')
		if err != nil {
			longRunningDone <- "ERROR: " + err.Error()
		} else {
			longRunningDone <- strings.TrimSpace(response)
		}
	}()

	// Give the request time to start processing
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	shutdownStart := time.Now()
	cancel()

	// The long-running request should complete successfully
	select {
	case response := <-longRunningDone:
		assert.Equal(t, "RESPONSE|ACCEPTED|Transaction processed", response)
		assert.Less(t, time.Since(shutdownStart), gracePeriod, "Request took longer than grace period")
	case <-time.After(gracePeriod + 500*time.Millisecond):
		t.Fatal("Long-running request did not complete within expected time")
	}

	// Try to send a new request on existing connection after shutdown started
	// This should be rejected
	time.Sleep(100 * time.Millisecond) // Let shutdown process
	fmt.Fprintf(conn2, "PAYMENT|100\n")
	response, err := bufio.NewReader(conn2).ReadString('\n')
	if err == nil {
		response = strings.TrimSpace(response)
		assert.Equal(t, "RESPONSE|REJECTED|Cancelled", response, "New requests should be cancelled")
	}
}

func TestGracePeriodExpiration(t *testing.T) {
	gracePeriod := 500 * time.Millisecond

	// Start test server with short grace period
	server, cancel, err := startTestServer(gracePeriod)
	require.NoError(t, err, "Failed to start test server")
	defer cancel()
	_ = server

	// Establish connection
	conn, err := net.Dial("tcp", ":8080")
	require.NoError(t, err, "Failed to connect to server")
	defer conn.Close()

	// Start a request that takes longer than grace period
	requestDone := make(chan string, 1)
	go func() {
		fmt.Fprintf(conn, "PAYMENT|1000\n") // 1000ms processing time > 500ms grace period
		response, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			requestDone <- "ERROR: " + err.Error()
		} else {
			requestDone <- strings.TrimSpace(response)
		}
	}()

	// Give request time to start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	shutdownStart := time.Now()
	cancel()

	// Request should be cancelled after grace period
	select {
	case response := <-requestDone:
		duration := time.Since(shutdownStart)
		assert.Equal(t, "RESPONSE|REJECTED|Cancelled", response, "Request should be cancelled")
		assert.LessOrEqual(t, duration, gracePeriod+200*time.Millisecond, "Cancellation should occur around grace period")
	case <-time.After(gracePeriod + 1*time.Second):
		t.Fatal("Request was not cancelled within expected time")
	}
}

func TestNewConnectionsDuringShutdown(t *testing.T) {
	gracePeriod := 1 * time.Second

	// Start test server
	server, cancel, err := startTestServer(gracePeriod)
	require.NoError(t, err, "Failed to start test server")
	defer cancel()
	_ = server

	// Trigger shutdown
	cancel()

	// Give shutdown time to start
	time.Sleep(100 * time.Millisecond)

	// Try to establish new connection - should fail
	conn, err := net.Dial("tcp", ":8080")
	if err == nil {
		conn.Close()
		t.Error("New connections should be rejected during shutdown")
	}
	// We expect this to fail, so no assertion needed if err != nil
}

func TestConnectionTracking(t *testing.T) {
	gracePeriod := 2 * time.Second
	server := NewServer(gracePeriod)

	// Test connection tracking methods
	// Create mock connections for testing
	listener, err := net.Listen("tcp", ":0") // Use random port
	require.NoError(t, err)
	defer listener.Close()

	// Simulate connections
	conn1, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn2.Close()

	// Accept the connections
	serverConn1, err := listener.Accept()
	require.NoError(t, err)
	defer serverConn1.Close()

	serverConn2, err := listener.Accept()
	require.NoError(t, err)
	defer serverConn2.Close()

	// Test adding connections
	server.addConnection(serverConn1)
	server.addConnection(serverConn2)

	// Verify connections are tracked
	server.mu.RLock()
	assert.Equal(t, 2, len(server.connections), "Should have 2 tracked connections")
	assert.True(t, server.connections[serverConn1], "Connection 1 should be tracked")
	assert.True(t, server.connections[serverConn2], "Connection 2 should be tracked")
	server.mu.RUnlock()

	// Test removing connection
	server.removeConnection(serverConn1)
	server.mu.RLock()
	assert.Equal(t, 1, len(server.connections), "Should have 1 tracked connection after removal")
	assert.False(t, server.connections[serverConn1], "Connection 1 should not be tracked")
	assert.True(t, server.connections[serverConn2], "Connection 2 should still be tracked")
	server.mu.RUnlock()

	// Test close all connections
	server.closeAllConnections()

	// Give time for connections to close
	time.Sleep(10 * time.Millisecond)

	// Connections should still be in map until removeConnection is called
	server.mu.RLock()
	assert.Equal(t, 1, len(server.connections), "Connection should remain in map until explicitly removed")
	server.mu.RUnlock()
}

func TestConcurrentRequests(t *testing.T) {
	gracePeriod := 2 * time.Second

	// Start test server
	server, cancel, err := startTestServer(gracePeriod)
	require.NoError(t, err, "Failed to start test server")
	defer cancel()
	_ = server

	numGoroutines := 10
	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)

	// Start multiple concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", ":8080")
			if err != nil {
				results <- fmt.Sprintf("Connection failed: %v", err)
				return
			}
			defer conn.Close()

			request := fmt.Sprintf("PAYMENT|%d", 50+id*10) // Variable processing times
			fmt.Fprintf(conn, request+"\n")

			response, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				results <- fmt.Sprintf("Read failed: %v", err)
				return
			}

			results <- strings.TrimSpace(response)
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	close(results)

	// Check all results
	successCount := 0
	for result := range results {
		if result == "RESPONSE|ACCEPTED|Transaction processed" {
			successCount++
		} else if strings.Contains(result, "failed") {
			t.Logf("Request failed: %s", result)
		}
	}

	assert.Equal(t, numGoroutines, successCount, "All concurrent requests should succeed")
}
