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

// Wallet manages Solana wallet operations
type Wallet struct {
	keypair    *solana.Keypair
	client     *Client
	logger     *utils.Logger
	cache      *sync.Map
	lastUpdate time.Time
	mu         sync.RWMutex
}

// WalletInfo contains wallet information
type WalletInfo struct {
	Address     string                 `json:"address"`
	Balance     uint64                 `json:"balance"`
	Tokens      []TokenBalance         `json:"tokens"`
	NFTs        []NFTInfo             `json:"nfts"`
	LastUpdated time.Time             `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// TokenBalance represents a token balance
type TokenBalance struct {
	Mint      string  `json:"mint"`
	Symbol    string  `json:"symbol"`
	Balance   uint64  `json:"balance"`
	Decimals  uint8   `json:"decimals"`
	Authority string  `json:"authority"`
}

// NFTInfo represents NFT information
type NFTInfo struct {
	Mint       string                 `json:"mint"`
	Name       string                 `json:"name"`
	URI        string                 `json:"uri"`
	Symbol     string                 `json:"symbol"`
	Collection string                 `json:"collection"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// NewWallet creates a new wallet instance
func NewWallet(client *Client, keypairData []byte) (*Wallet, error) {
	keypair, err := solana.KeypairFromBytes(keypairData)
	if err != nil {
		return nil, fmt.Errorf("failed to create keypair: %w", err)
	}

	return &Wallet{
		keypair:    keypair,
		client:     client,
		logger:     utils.NewLogger(),
		cache:      &sync.Map{},
		lastUpdate: time.Now(),
	}, nil
}

// GetAddress returns the wallet's public address
func (w *Wallet) GetAddress() string {
	return w.keypair.PublicKey.String()
}

// GetBalance returns the wallet's SOL balance
func (w *Wallet) GetBalance(ctx context.Context) (uint64, error) {
	balance, err := w.client.GetBalance(ctx, w.GetAddress())
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// GetInfo returns comprehensive wallet information
func (w *Wallet) GetInfo(ctx context.Context) (*WalletInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	balance, err := w.GetBalance(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := w.getTokenBalances(ctx)
	if err != nil {
		return nil, err
	}

	nfts, err := w.getNFTs(ctx)
	if err != nil {
		return nil, err
	}

	info := &WalletInfo{
		Address:     w.GetAddress(),
		Balance:     balance,
		Tokens:      tokens,
		NFTs:        nfts,
		LastUpdated: time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	w.lastUpdate = time.Now()
	return info, nil
}

// SignTransaction signs a transaction
func (w *Wallet) SignTransaction(transaction *solana.Transaction) error {
	_, err := transaction.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(w.keypair.PublicKey) {
			return &w.keypair.PrivateKey
		}
		return nil
	})
	return err
}

// SendSOL sends SOL to a recipient
func (w *Wallet) SendSOL(ctx context.Context, recipient string, amount uint64) (string, error) {
	recipientPubKey, err := solana.PublicKeyFromBase58(recipient)
	if err != nil {
		return "", fmt.Errorf("invalid recipient address: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			solana.NewInstruction(
				solana.SystemProgramID,
				[]byte{2, 0, 0, 0}, // Transfer instruction
				[]solana.AccountMeta{
					{PublicKey: w.keypair.PublicKey, IsSigner: true, IsWritable: true},
					{PublicKey: recipientPubKey, IsSigner: false, IsWritable: true},
				},
				amount,
			),
		},
		w.keypair.PublicKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	if err := w.SignTransaction(tx); err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	signature, err := w.client.SendTransaction(ctx, serializedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return signature, nil
}

// getTokenBalances retrieves all token balances
func (w *Wallet) getTokenBalances(ctx context.Context) ([]TokenBalance, error) {
	accounts, err := w.client.rpcClient.GetTokenAccountsByOwner(
		ctx,
		w.keypair.PublicKey,
		&rpc.GetTokenAccountsConfig{
			ProgramId: solana.TokenProgramID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get token accounts: %w", err)
	}

	var balances []TokenBalance
	for _, account := range accounts.Value {
		var data solana.TokenAccount
		if err := data.UnmarshalBinary(account.Account.Data.GetBinary()); err != nil {
			continue
		}

		balance := TokenBalance{
			Mint:      data.Mint.String(),
			Balance:   data.Amount,
			Decimals: data.Decimals,
			Authority: data.Owner.String(),
		}
		balances = append(balances, balance)
	}

	return balances, nil
}

// getNFTs retrieves all NFTs owned by the wallet
func (w *Wallet) getNFTs(ctx context.Context) ([]NFTInfo, error) {
	// This is a simplified implementation
	// In a real application, you would need to:
	// 1. Query Metaplex accounts
	// 2. Fetch metadata from URIs
	// 3. Filter for actual NFTs
	return []NFTInfo{}, nil
}

// ExportPrivateKey exports the private key (use with caution)
func (w *Wallet) ExportPrivateKey() []byte {
	return w.keypair.PrivateKey[:]
}

// ImportPrivateKey imports a private key
func ImportPrivateKey(privateKeyBytes []byte, client *Client) (*Wallet, error) {
	return NewWallet(client, privateKeyBytes)
}

// CreateNewWallet creates a new random wallet
func CreateNewWallet(client *Client) (*Wallet, error) {
	keypair := solana.NewWallet()
	return NewWallet(client, keypair.PrivateKey[:])
}