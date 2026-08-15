package sigilcrypto

import (
	"testing"
)

func BenchmarkEncrypt(b *testing.B) {
	data := make([]byte, 1024)
	password := "testpassword123"

	b.ResetTimer()
	for range b.N {
		_, _ = Encrypt(data, password)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	data := make([]byte, 1024)
	password := "testpassword123"
	encrypted, _ := Encrypt(data, password)

	b.ResetTimer()
	for range b.N {
		_, _ = Decrypt(encrypted, password)
	}
}

func BenchmarkRandomBytes32(b *testing.B) {
	for range b.N {
		_, _ = RandomBytes(32)
	}
}

func BenchmarkRandomBytes64(b *testing.B) {
	for range b.N {
		_, _ = RandomBytes(64)
	}
}
