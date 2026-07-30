package btc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPStatusError_Error(t *testing.T) {
	t.Parallel()

	err := &httpStatusError{Code: 400, Body: "too many utxos"}
	assert.Equal(t, "esplora: status 400: too many utxos", err.Error())

	// It satisfies the error interface.
	var _ error = err
	assert.Equal(t, "esplora: status 500: ", (&httpStatusError{Code: 500}).Error())
}
