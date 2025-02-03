package unit

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labs-alone/alone-main/internal/solana"
	"github.com/labs-alone/alone-main/internal/utils"
)

type mockConnection struct {
	responses map[string]interface{}
	errors    map[string]error
}

func setupTestClient(t *testing.T) (*solana.Client, *utils.Config) {
	config, err := utils.LoadConfig("../../config/test.yaml")
	require.NoError(t, err)

	client, err := solana.NewClient(config.Solana)
	require.NoError(t, err)

	return client, config
}

func TestClientInitialization(t *testing.T) {
	client, _ := setupTestClient(t)

	assert.NotNil(t, client)
	assert.Equal(t, "connected", client.Status())

	metrics := client.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, uint64(0), metrics.RequestCount)
}

func TestGetBalance(t *testing.T) {
	client, _ := setupTestClient(t)

	testCases := []struct {
		name        string
		address     string
		expectError bool
		expected    uint64
	}{
		{
			name:        "Valid Address",
			address:     "valid_solana_address",
			expectError: false,
			expected:    1000000000,
		},
		{
			name:        "Invalid Address",
			address:     "invalid_address",
			expectError: true,
			expected:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			balance, err := client.GetBalance(context.Background(), tc.address)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, balance)
			}
		})
	}
}

func TestSendTransaction(t *testing.T) {
	client, _ := setupTestClient(t)

	testCases := []struct {
		name        string
		from       string
		to         string
		amount     uint64
		expectError bool
	}{
		{
			name:        "Valid Transaction",
			from:       "sender_address",
			to:         "recipient_address",
			amount:     1000000,
			expectError: false,
		},
		{
			name:        "Invalid Amount",
			from:       "sender_address",
			to:         "recipient_address",
			amount:     0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signature, err := client.SendTransaction(
				context.Background(),
				tc.from,
				tc.to,
				tc.amount,
			)
			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, signature)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, signature)
			}
		})
	}
}

func TestGetAccountInfo(t *testing.T) {
	client, _ := setupTestClient(t)

	testCases := []struct {
		name        string
		address     string
		expectError bool
	}{
		{
			name:        "Valid Account",
			address:     "valid_account_address",
			expectError: false,
		},
		{
			name:        "Invalid Account",
			address:     "invalid_address",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := client.GetAccountInfo(context.Background(), tc.address)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, info)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, info)
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	client, _ := setupTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numRequests = 50
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := client.GetBalance(ctx, "test_address")
			results <- err
		}(i)
	}

	for i := 0; i < numRequests; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}
	}
}

func TestTransactionConfirmation(t *testing.T) {
	client, _ := setupTestClient(t)

	signature := "test_signature"
	
	testCases := []struct {
		name        string
		commitment  string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "Quick Confirmation",
			commitment:  "confirmed",
			timeout:     time.Second,
			expectError: false,
		},
		{
			name:        "Timeout",
			commitment:  "finalized",
			timeout:     time.Millisecond,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			err := client.ConfirmTransaction(ctx, signature, tc.commitment)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	client, _ := setupTestClient(t)

	testCases := []struct {
		name        string
		operation   func() error
		expectError bool
	}{
		{
			name: "Invalid RPC Request",
			operation: func() error {
				_, err := client.GetBalance(context.Background(), "")
				return err
			},
			expectError: true,
		},
		{
			name: "Context Cancellation",
			operation: func() error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := client.GetBalance(ctx, "address")
				return err
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.operation()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetrics(t *testing.T) {
	client, _ := setupTestClient(t)

	// Make some requests
	for i := 0; i < 5; i++ {
		_, _ = client.GetBalance(context.Background(), "test_address")
	}

	metrics := client.GetMetrics()
	assert.Equal(t, uint64(5), metrics.RequestCount)
	assert.NotZero(t, metrics.AverageLatency)
	assert.NotZero(t, metrics.LastRequest)
}

func BenchmarkGetBalance(b *testing.B) {
	client, _ := setupTestClient(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetBalance(ctx, "test_address")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSendTransaction(b *testing.B) {
	client, _ := setupTestClient(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SendTransaction(ctx, "from", "to", 1000000)
		if err != nil {
			b.Fatal(err)
		}
	}
}