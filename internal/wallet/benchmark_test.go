package wallet

import (
	"testing"

	"github.com/mrz1836/sigil/internal/testutil"
)

func BenchmarkGenerateMnemonic12(b *testing.B) {
	for range b.N {
		_, _ = GenerateMnemonic(12)
	}
}

func BenchmarkGenerateMnemonic24(b *testing.B) {
	for range b.N {
		_, _ = GenerateMnemonic(24)
	}
}

func BenchmarkDeriveAddressETH(b *testing.B) {
	mnemonic := testutil.TestMnemonic
	seed, _ := MnemonicToSeed(mnemonic, "")
	defer ZeroBytes(seed)

	b.ResetTimer()
	for i := range b.N {
		_, _ = DeriveAddress(seed, ChainETH, 0, uint32(i%100))
	}
}

func BenchmarkDeriveAddressBSV(b *testing.B) {
	mnemonic := testutil.TestMnemonic
	seed, _ := MnemonicToSeed(mnemonic, "")
	defer ZeroBytes(seed)

	b.ResetTimer()
	for i := range b.N {
		_, _ = DeriveAddress(seed, ChainBSV, 0, uint32(i%100))
	}
}

func BenchmarkValidateMnemonic(b *testing.B) {
	mnemonic := testutil.TestMnemonic
	b.ResetTimer()
	for range b.N {
		_ = ValidateMnemonic(mnemonic)
	}
}

func BenchmarkMnemonicToSeed(b *testing.B) {
	mnemonic := testutil.TestMnemonic
	b.ResetTimer()
	for range b.N {
		seed, _ := MnemonicToSeed(mnemonic, "")
		ZeroBytes(seed)
	}
}

func BenchmarkDerivePrivateKey(b *testing.B) {
	mnemonic := testutil.TestMnemonic
	seed, _ := MnemonicToSeed(mnemonic, "")
	defer ZeroBytes(seed)

	b.ResetTimer()
	for i := range b.N {
		key, _ := DerivePrivateKey(seed, ChainETH, 0, uint32(i%100))
		ZeroBytes(key)
	}
}
