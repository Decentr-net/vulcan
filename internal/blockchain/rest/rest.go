// Package rest ...
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TokenResponse ...
type TokenResponse struct {
	Height string `json:"height"`
	Result struct {
		Balance      sdk.Dec `json:"balance"`
		BalanceDelta sdk.Dec `json:"balanceDelta"`
	} `json:"result"`
}

// BlockchainRESTClient is a client to the blockchain REST interface.
type BlockchainRESTClient struct {
	baseURL string
	client  *http.Client
}

// NewBlockchainRESTClient ...
func NewBlockchainRESTClient(baseURL string) *BlockchainRESTClient {
	return &BlockchainRESTClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTokenBalance returns token balance (PDV) of the given address.
func (b *BlockchainRESTClient) GetTokenBalance(ctx context.Context, address string) (*TokenResponse, error) {
	url := fmt.Sprintf("%s/token/balance/%s", b.baseURL, address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make a request: %w", err)
	}
	defer resp.Body.Close() // nolint

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode) // nolint:err113
	}

	var out TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	return &out, nil
}
