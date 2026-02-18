package output

import (
	"io"
	"os"

	"github.com/mdp/qrterminal/v3"
	"golang.org/x/term"
	"rsc.io/qr"
)

// QRConfig configures QR code rendering.
type QRConfig struct {
	// Level is the error correction level.
	Level qr.Level
	// QuietZone is the number of empty blocks around the QR code.
	QuietZone int
	// HalfBlocks uses half-height blocks for a more compact display.
	HalfBlocks bool
}

// DefaultQRConfig returns sensible defaults for terminal QR rendering.
func DefaultQRConfig() QRConfig {
	return QRConfig{
		Level:      qr.L, // Low error correction is sufficient for addresses
		QuietZone:  1,
		HalfBlocks: true, // Compact display for terminals
	}
}

// CanRenderQR checks if the output writer is a terminal suitable for QR rendering.
func CanRenderQR(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // G115: Fd() returns uintptr, safe conversion for term.IsTerminal
}

// RenderQR renders a QR code to the writer if it's a terminal.
// Returns without error if the writer is not a terminal (no output is produced).
func RenderQR(w io.Writer, data string, cfg QRConfig) error {
	if !CanRenderQR(w) {
		return nil
	}

	config := qrterminal.Config{
		Level:          cfg.Level,
		Writer:         w,
		QuietZone:      cfg.QuietZone,
		HalfBlocks:     cfg.HalfBlocks,
		BlackChar:      qrterminal.BLACK_BLACK,
		WhiteChar:      qrterminal.WHITE_WHITE,
		WhiteBlackChar: qrterminal.WHITE_BLACK,
		BlackWhiteChar: qrterminal.BLACK_WHITE,
	}

	qrterminal.GenerateWithConfig(data, config)
	return nil
}
