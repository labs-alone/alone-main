package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labs-alone/alone-main/internal/core"
	"github.com/labs-alone/alone-main/internal/utils"
)

type mockState struct {
	status string
	data   map[string]interface{}
}

func setupTestEngine(t *testing.T) (*core.Engine, *utils.Config) {
	config, err := utils.LoadConfig("../../config/test.yaml")
	require.NoError(t, err)

	engine, err := core.NewEngine(config)
	require.NoError(t, err)

	return engine, config
}

func TestEngineInitialization(t *testing.T) {
	engine, _ := setupTestEngine(t)

	assert.NotNil(t, engine)
	assert.Equal(t, "ready", engine.Status())

	metrics := engine.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, uint64(0), metrics.RequestCount)
}

func TestEngineStateManagement(t *testing.T) {
	engine, _ := setupTestEngine(t)

	testCases := []struct {
		name     string
		state    mockState
		expected bool
	}{
		{
			name: "Valid State",
			state: mockState{
				status: "active",
				data: map[string]interface{}{
					"key": "value",
				},
			},
			expected: true,
		},
		{
			name: "Empty State",
			state: mockState{
				status: "",
				data:   nil,
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.UpdateState(tc.state.status, tc.state.data)
			if tc.expected {
				assert.NoError(t, err)
				state := engine.GetState()
				assert.Equal(t, tc.state.status, state.Status)
				assert.Equal(t, tc.state.data, state.Data)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestEngineRequestProcessing(t *testing.T) {
	engine, _ := setupTestEngine(t)

	testCases := []struct {
		name        string
		request     *core.Request
		expectError bool
	}{
		{
			name: "Valid Request",
			request: &core.Request{
				ID:      "test-1",
				Type:    "test",
				Payload: map[string]interface{}{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "Invalid Request",
			request: &core.Request{
				ID:      "",
				Type:    "",
				Payload: nil,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.ProcessRequest(tc.request)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.request.ID, result.RequestID)
			}
		})
	}
}

func TestEngineConcurrency(t *testing.T) {
	engine, _ := setupTestEngine(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numRequests = 100
	results := make(chan error, numRequests)

	// Send concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			request := &core.Request{
				ID:      fmt.Sprintf("test-%d", id),
				Type:    "test",
				Payload: map[string]interface{}{"value": id},
			}

			_, err := engine.ProcessRequest(request)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("Test timed out")
		}
	}

	// Verify metrics
	metrics := engine.GetMetrics()
	assert.Equal(t, uint64(numRequests), metrics.RequestCount)
}

func TestEngineErrorHandling(t *testing.T) {
	engine, _ := setupTestEngine(t)

	testCases := []struct {
		name        string
		operation   func() error
		expectError bool
	}{
		{
			name: "Invalid State Update",
			operation: func() error {
				return engine.UpdateState("", nil)
			},
			expectError: true,
		},
		{
			name: "Invalid Request",
			operation: func() error {
				_, err := engine.ProcessRequest(nil)
				return err
			},
			expectError: true,
		},
		{
			name: "Invalid Configuration",
			operation: func() error {
				return engine.UpdateConfig(nil)
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

func TestEngineMetrics(t *testing.T) {
	engine, _ := setupTestEngine(t)

	// Process some requests
	for i := 0; i < 5; i++ {
		request := &core.Request{
			ID:      fmt.Sprintf("test-%d", i),
			Type:    "test",
			Payload: map[string]interface{}{"value": i},
		}
		_, err := engine.ProcessRequest(request)
		require.NoError(t, err)
	}

	metrics := engine.GetMetrics()
	assert.Equal(t, uint64(5), metrics.RequestCount)
	assert.Equal(t, uint64(0), metrics.ErrorCount)
	assert.NotZero(t, metrics.AverageLatency)
	assert.NotZero(t, metrics.LastRequest)
}

func TestEngineShutdown(t *testing.T) {
	engine, _ := setupTestEngine(t)

	// Start some background work
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				request := &core.Request{
					ID:      "test",
					Type:    "test",
					Payload: map[string]interface{}{"key": "value"},
				}
				engine.ProcessRequest(request)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Let it run for a bit
	time.Sleep(500 * time.Millisecond)

	// Shutdown
	err := engine.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "shutdown", engine.Status())

	// Verify no new requests are accepted
	_, err = engine.ProcessRequest(&core.Request{
		ID:   "test-after-shutdown",
		Type: "test",
	})
	assert.Error(t, err)
}

func TestEngineConfiguration(t *testing.T) {
	engine, config := setupTestEngine(t)

	// Test configuration updates
	newConfig := *config
	newConfig.LogLevel = "debug"

	err := engine.UpdateConfig(&newConfig)
	assert.NoError(t, err)

	currentConfig := engine.GetConfig()
	assert.Equal(t, "debug", currentConfig.LogLevel)
}

func BenchmarkEngineRequestProcessing(b *testing.B) {
	engine, _ := setupTestEngine(b)

	request := &core.Request{
		ID:      "bench-test",
		Type:    "test",
		Payload: map[string]interface{}{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.ProcessRequest(request)
		if err != nil {
			b.Fatal(err)
		}
	}
}