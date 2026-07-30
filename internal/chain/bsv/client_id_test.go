package bsv

import (
	"context"
	"testing"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestClient_ID(t *testing.T) {
	t.Parallel()
	client := NewClient(context.Background(), &ClientOptions{WOCClient: &mockWOCClient{}})
	assert.Equal(t, chain.BSV, client.ID())
}

func TestClient_GetWOCClient(t *testing.T) {
	t.Parallel()
	woc := &mockWOCClient{}
	client := NewClient(context.Background(), &ClientOptions{WOCClient: woc})
	require.NotNil(t, client.GetWOCClient())
	assert.Same(t, woc, client.GetWOCClient())
}

func TestMapNetwork(t *testing.T) {
	t.Parallel()
	assert.Equal(t, whatsonchain.NetworkTest, mapNetwork(NetworkTestnet))
	assert.Equal(t, whatsonchain.NetworkMain, mapNetwork(NetworkMainnet))
	// Unknown networks fall back to mainnet.
	assert.Equal(t, whatsonchain.NetworkMain, mapNetwork(Network("bogus")))
}

func TestArcAPIError_Error(t *testing.T) {
	t.Parallel()

	// With Detail set, both detail and title appear.
	withDetail := &arcAPIError{Title: "Bad Request", Status: 400, Detail: "invalid tx"}
	assert.Equal(t, "arc: invalid tx (status 400: Bad Request)", withDetail.Error())

	// Without Detail, only the title is shown.
	noDetail := &arcAPIError{Title: "Server Error", Status: 500}
	assert.Equal(t, "arc: Server Error (status 500)", noDetail.Error())
}
