package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func testKey() []byte {
	return bytes.Repeat([]byte("k"), 32)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey()

	data, err := encrypt(key, "hello")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decrypt(key, data)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	key := testKey()

	a, err := encrypt(key, "same text")
	if err != nil {
		t.Fatal(err)
	}
	b, err := encrypt(key, "same text")
	if err != nil {
		t.Fatal(err)
	}

	// same key and same text must still give different output
	if bytes.Equal(a, b) {
		t.Error("two encryptions of the same text were identical")
	}
}

func TestDecryptRejectsTampering(t *testing.T) {
	key := testKey()

	data, err := encrypt(key, "hello")
	if err != nil {
		t.Fatal(err)
	}

	data[len(data)-1] ^= 0xFF // flip bits in the last byte

	if _, err := decrypt(key, data); err == nil {
		t.Error("decrypt accepted data that was changed")
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	data, err := encrypt(testKey(), "hello")
	if err != nil {
		t.Fatal(err)
	}

	otherKey := bytes.Repeat([]byte("x"), 32)
	if _, err := decrypt(otherKey, data); err == nil {
		t.Error("decrypt worked with the wrong key")
	}
}

func TestDecryptRejectsShortData(t *testing.T) {
	if _, err := decrypt(testKey(), []byte("tiny")); err == nil {
		t.Error("decrypt accepted data shorter than a nonce")
	}
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"not base64", "%%%not-base64%%%", true},
		{"wrong length", base64.StdEncoding.EncodeToString([]byte("short")), true},
		{"valid", base64.StdEncoding.EncodeToString(testKey()), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseKey(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewIDIsUniqueAndLongEnough(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		id, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		// 16 random bytes in base64 (no padding) is 22 characters
		if len(id) != 22 {
			t.Fatalf("id %q has length %d, want 22", id, len(id))
		}
		if seen[id] {
			t.Fatalf("id %q was generated twice", id)
		}
		seen[id] = true
	}
}