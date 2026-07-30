package address

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddressType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  AddressType
		want string
	}{
		{"receive", Receive, "receive"},
		{"change", Change, "change"},
		{"all types", AllTypes, "all"},
		// Any value outside the defined enum falls through to the default arm.
		{"unknown falls through to default", AddressType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.String())
		})
	}
}
