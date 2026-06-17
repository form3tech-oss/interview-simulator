package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	go Start()

	// wait of the server to be ready
	time.Sleep(time.Second)

	code := m.Run()

	os.Exit(code)
}

func TestSchemeSimulator(t *testing.T) {
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
			maxDuration:    151 * time.Millisecond,
		},

		{
			name:           "Invalid Request Format",
			input:          "INVALID|100",
			expectedOutput: "RESPONSE|REJECTED|Invalid request",
			maxDuration:    10 * time.Millisecond,
		},
		{
			name:           "Large Amount",
			input:          "PAYMENT|20000",
			expectedOutput: "RESPONSE|ACCEPTED|Transaction processed",
			minDuration:    10 * time.Second,
			maxDuration:    10*time.Second + 50*time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ":8080")
			require.NoError(t, err, "Failed to connect to server")
			defer conn.Close()

			_, err = fmt.Fprintf(conn, "%s\n", tt.input)
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
