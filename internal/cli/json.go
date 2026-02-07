package cli

import (
	"encoding/json"
	"io"
)

// writeJSON encodes the value as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
