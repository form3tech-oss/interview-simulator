package main

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// handleRequest processes individual payment requests
func (s *Server) handleRequest(ctx context.Context, request string) string {
	parts := strings.Split(request, "|")
	if len(parts) != 2 || parts[0] != "PAYMENT" {
		return "RESPONSE|REJECTED|Invalid request"
	}

	amount, err := strconv.Atoi(parts[1])
	if err != nil {
		return "RESPONSE|REJECTED|Invalid amount"
	}

	if amount > 100 {
		processingTime := amount
		if amount > 10000 {
			processingTime = 10000
		}

		// Use context-aware sleep that can be interrupted
		sleepDuration := time.Duration(processingTime) * time.Millisecond
		timer := time.NewTimer(sleepDuration)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Sleep completed normally
		case <-ctx.Done():
			// Context cancelled during processing, return cancellation
			return "RESPONSE|REJECTED|Cancelled"
		}
	}

	return "RESPONSE|ACCEPTED|Transaction processed"
}
