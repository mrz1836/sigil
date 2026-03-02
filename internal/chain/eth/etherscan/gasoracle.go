package etherscan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

var (
	errEmptyGwei       = errors.New("empty gwei value")
	errInvalidGwei     = errors.New("invalid gwei value")
	errInvalidFracGwei = errors.New("invalid fractional gwei value")
)

// ErrGasOracleFailed indicates the gas oracle API returned an error.
var ErrGasOracleFailed = &sigilerr.SigilError{
	Code:     "ETHERSCAN_GAS_ORACLE_FAILED",
	Message:  "Etherscan gas oracle returned an error",
	ExitCode: sigilerr.ExitGeneral,
}

// gasOracleAPIResponse is the Etherscan response for the gas oracle endpoint.
// Unlike apiResponse, the Result field is a JSON object, not a string.
type gasOracleAPIResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  GasOracleResult `json:"result"`
}

// GasOracleResult contains gas prices from the Etherscan gas oracle.
type GasOracleResult struct {
	SafeGasPrice    string `json:"SafeGasPrice"`    // Gwei, maps to "slow"
	ProposeGasPrice string `json:"ProposeGasPrice"` // Gwei, maps to "medium"
	FastGasPrice    string `json:"FastGasPrice"`    // Gwei, maps to "fast"
}

// GetGasOracle fetches current gas prices from the Etherscan gas tracker.
func (c *Client) GetGasOracle(ctx context.Context) (*GasOracleResult, error) {
	if err := c.rateLimiter.Wait(ctx, "etherscan"); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	params := url.Values{
		"module": {"gastracker"},
		"action": {"gasoracle"},
	}
	params.Set("chainid", c.chainID)

	reqURL := fmt.Sprintf("%s/api?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq) //nolint:gosec // URL is constructed from validated config
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		return nil, sigilerr.WithDetails(ErrAPIError, map[string]string{
			"status": fmt.Sprintf("%d", resp.StatusCode),
			"body":   truncateBody(string(body), 512),
		})
	}

	var apiResp gasOracleAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing gas oracle response: %w", err)
	}

	if err := validateGasOracleStatus(&apiResp); err != nil {
		return nil, err
	}

	return &apiResp.Result, nil
}

// validateGasOracleStatus checks the API response status and returns an appropriate error.
func validateGasOracleStatus(apiResp *gasOracleAPIResponse) error {
	if apiResp.Status == "1" {
		return nil
	}
	if apiResp.Message == "NOTOK" || strings.Contains(fmt.Sprintf("%v", apiResp.Result), "Max rate limit reached") {
		return ErrRateLimited
	}
	return sigilerr.WithDetails(ErrGasOracleFailed, map[string]string{
		"message": apiResp.Message,
	})
}

// GasPriceAdapter adapts the Etherscan gas oracle to the eth.GasPriceOracle interface.
type GasPriceAdapter struct {
	client *Client
}

// NewGasPriceAdapter creates a new adapter that satisfies eth.GasPriceOracle.
func NewGasPriceAdapter(c *Client) *GasPriceAdapter {
	return &GasPriceAdapter{client: c}
}

// GetGasPrices returns gas prices for slow/medium/fast speeds in wei.
func (a *GasPriceAdapter) GetGasPrices(ctx context.Context) (slow, medium, fast *big.Int, err error) {
	result, err := a.client.GetGasOracle(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	slow, err = gweiToWei(result.SafeGasPrice)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing SafeGasPrice %q: %w", result.SafeGasPrice, err)
	}

	medium, err = gweiToWei(result.ProposeGasPrice)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing ProposeGasPrice %q: %w", result.ProposeGasPrice, err)
	}

	fast, err = gweiToWei(result.FastGasPrice)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing FastGasPrice %q: %w", result.FastGasPrice, err)
	}

	return slow, medium, fast, nil
}

// gweiToWei converts a Gwei string (integer or decimal like "7.5") to wei (*big.Int).
func gweiToWei(gwei string) (*big.Int, error) {
	if gwei == "" {
		return nil, errEmptyGwei
	}

	// Split on decimal point
	parts := strings.SplitN(gwei, ".", 2)

	// Parse the integer part
	intPart, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, fmt.Errorf("%w: %s", errInvalidGwei, gwei)
	}

	// Convert integer part to wei (multiply by 10^9)
	gweiMultiplier := new(big.Int).SetUint64(1_000_000_000)
	result := new(big.Int).Mul(intPart, gweiMultiplier)

	// Handle decimal part if present
	if len(parts) == 2 && parts[1] != "" {
		fracStr := parts[1]
		// Pad or truncate to 9 decimal places (nano precision for gwei → wei)
		if len(fracStr) > 9 {
			fracStr = fracStr[:9]
		}
		for len(fracStr) < 9 {
			fracStr += "0"
		}

		fracPart, ok := new(big.Int).SetString(fracStr, 10)
		if !ok {
			return nil, fmt.Errorf("%w: %s", errInvalidFracGwei, gwei)
		}
		result.Add(result, fracPart)
	}

	return result, nil
}
