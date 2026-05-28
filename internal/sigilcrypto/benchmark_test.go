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

func BenchmarkSecureBytesCreate(b *testing.B) {
	for range b.N {
		sb, _ := NewSecureBytes(64)
		sb.Destroy()
	}
}

func BenchmarkSecureBytesFromSlice(b *testing.B) {
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for range b.N {
		sb, _ := SecureBytesFromSlice(data)
		sb.Destroy()
	}
}
