package discovery

import (
	"context"
)

// RefreshBatch refreshes multiple addresses and returns individual results.
// Addresses are refreshed sequentially or concurrently based on req.Concurrent.
func (s *Service) RefreshBatch(ctx context.Context, req *RefreshRequest) ([]RefreshResult, error) {
	results := make([]RefreshResult, 0, len(req.Addresses))

	// Sequential refresh (default)
	for _, addr := range req.Addresses {
		// Create per-address context with timeout
		addrCtx := ctx
		var cancel context.CancelFunc
		if req.Timeout > 0 {
			addrCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		}

		// Refresh the address
		err := s.refreshAddress(addrCtx, req.ChainID, addr)

		if cancel != nil {
			cancel()
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			results = append(results, RefreshResult{
				Address: addr,
				Success: false,
				Error:   ctx.Err(),
			})
			break
		}

		results = append(results, RefreshResult{
			Address: addr,
			Success: err == nil,
			Error:   err,
		})
	}

	return results, nil
}
