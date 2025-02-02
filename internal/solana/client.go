package solana

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/labs-alone/alone-main/internal/utils"
)

// ClientConfig holds the Solana client configuration
type ClientConfig struct {
	Endpoint    string        `json:"endpoint"`
	Commitment  string        `json:"commitment"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int          `json:"max_retries"`
	Environment string        `json:"environment"`
}

// Client manages Solana blockchain interactions
type Client struct {
	config     *ClientConfig
	rpcClient  *rpc.Client
	wsClient   *rpc.WsClient
	logger     *utils.Logger
	cache      *sync.Map
	subscriptions map[string]*Subscription
	mu         sync.RWMutex
}

// Subscription represents a websocket subscription
type Subscription struct {
	ID       string
	Type     string
	Callback func(interface{}) error
	Active   bool
}

// TransactionInfo holds processed transaction data
type TransactionInfo struct {
	Signature     string                 `json:"signature"`
	Status        string                 `json:"status"`
	BlockTime     int64                  `json:"block_time"`
	Confirmations uint64                 `json:"confirmations"`
	Fee           uint64                 `json:"fee"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// NewClient creates a new Solana client instance
func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		config = &ClientConfig{
			Endpoint:    rpc.DevnetRPCEndpoint,
			Commitment:  rpc.CommitmentFinalized,
			Timeout:     time.Second * 30,
			MaxRetries:  3,
			Environment: "devnet",
		}
	}

	rpcClient := rpc.New(config.Endpoint)

	wsEndpoint := fmt.Sprintf("ws%s", config.Endpoint[4:])
	wsClient, err := rpc.NewWsClient(wsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket client: %w", err)
	}

	return &Client{
		config:        config,
		rpcClient:     rpcClient,
		wsClient:      wsClient,
		logger:        utils.NewLogger(),
		cache:         &sync.Map{},
		subscriptions: make(map[string]*Subscription),
	}, nil
}

// GetBalance retrieves the balance for a given address
func (c *Client) GetBalance(ctx context.Context, address string) (uint64, error) {
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("invalid address: %w", err)
	}

	balance, err := c.rpcClient.GetBalance(
		ctx,
		pubKey,
		rpc.CommitmentConfig{Commitment: c.config.Commitment},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance.Value, nil
}

// GetTransaction retrieves transaction information
func (c *Client) GetTransaction(ctx context.Context, signature string) (*TransactionInfo, error) {
	// Check cache first
	if cached, ok := c.cache.Load(signature); ok {
		return cached.(*TransactionInfo), nil
	}

	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	tx, err := c.rpcClient.GetTransaction(ctx, sig)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	info := &TransactionInfo{
		Signature:     signature,
		Status:        "confirmed",
		BlockTime:     tx.BlockTime,
		Confirmations: tx.Confirmations,
		Fee:           tx.Meta.Fee,
		Metadata:      make(map[string]interface{}),
	}

	// Cache the result
	c.cache.Store(signature, info)

	return info, nil
}

// SubscribeToProgram subscribes to program account changes
func (c *Client) SubscribeToProgram(programID string, callback func(interface{}) error) (string, error) {
	pubKey, err := solana.PublicKeyFromBase58(programID)
	if err != nil {
		return "", fmt.Errorf("invalid program ID: %w", err)
	}

	sub := &Subscription{
		ID:       utils.GenerateID(),
		Type:     "program",
		Callback: callback,
		Active:   true,
	}

	err = c.wsClient.ProgramSubscribe(
		pubKey,
		rpc.CommitmentConfig{Commitment: c.config.Commitment},
		func(result interface{}) error {
			if sub.Active {
				return callback(result)
			}
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to program: %w", err)
	}

	c.mu.Lock()
	c.subscriptions[sub.ID] = sub
	c.mu.Unlock()

	return sub.ID, nil
}

// UnsubscribeFromProgram unsubscribes from program updates
func (c *Client) UnsubscribeFromProgram(subscriptionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sub, exists := c.subscriptions[subscriptionID]
	if !exists {
		return fmt.Errorf("subscription not found")
	}

	sub.Active = false
	delete(c.subscriptions, subscriptionID)

	return nil
}

// SendTransaction sends a signed transaction
func (c *Client) SendTransaction(ctx context.Context, transaction []byte) (string, error) {
	tx, err := solana.TransactionFromDecoder(solana.NewBinDecoder(transaction))
	if err != nil {
		return "", fmt.Errorf("failed to decode transaction: %w", err)
	}

	sig, err := c.rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return sig.String(), nil
}

// GetAccountInfo retrieves account information
func (c *Client) GetAccountInfo(ctx context.Context, address string) (map[string]interface{}, error) {
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	info, err := c.rpcClient.GetAccountInfo(ctx, pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(info.Value.Data.GetBinary(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse account data: %w", err)
	}

	return result, nil
}

// Close closes the client connections
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close all active subscriptions
	for _, sub := range c.subscriptions {
		sub.Active = false
	}
	c.subscriptions = make(map[string]*Subscription)

	if err := c.wsClient.Close(); err != nil {
		return fmt.Errorf("failed to close websocket client: %w", err)
	}

	return nil
}