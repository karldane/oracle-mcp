package presidio

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestPseudonymiseOperatorAnonymiseDecryptRoundTrip(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")} // 32 bytes
	value := "test@example.com"
	result := op.Anonymise(value, 0, len(value), EntityEmailAddress)

	if !strings.HasPrefix(result, "pii:") {
		t.Fatalf("Expected result to start with 'pii:', got %s", result)
	}

	plaintext, err := op.Decrypt(result)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if plaintext != value {
		t.Fatalf("Round-trip failed: expected %s, got %s", value, plaintext)
	}
}

func TestPseudonymiseOperatorAnonymiseProducesUnifiedPrefix(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := "test@example.com"
	result := op.Anonymise(value, 0, len(value), EntityEmailAddress)

	if !strings.HasPrefix(result, "pii:") {
		t.Fatalf("Expected result to start with 'pii:', got %s", result)
	}
}

func TestPseudonymiseOperatorAnonymiseDeterministic(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	op1 := &PseudonymiseOperator{Key: key}
	op2 := &PseudonymiseOperator{Key: key}
	value := "test@example.com"

	result1 := op1.Anonymise(value, 0, len(value), EntityEmailAddress)
	result2 := op2.Anonymise(value, 0, len(value), EntityEmailAddress)

	if result1 != result2 {
		t.Fatalf("Expected deterministic output, got %s and %s", result1, result2)
	}
}

func TestPseudonymiseOperatorAnonymiseEmptyKey(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte{}}
	value := "test@example.com"
	result := op.Anonymise(value, 0, len(value), EntityEmailAddress)

	if result != "[pseudonymise requires key]" {
		t.Fatalf("Expected '[pseudonymise requires key]' for empty key, got %s", result)
	}
}

func TestPseudonymiseOperatorAnonymiseEmptyKeyZeroLength(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte{}}
	value := "test@example.com"
	result := op.Anonymise(value, 0, 0, EntityEmailAddress) // zero-length span

	if result != value {
		t.Fatalf("Expected unchanged value for zero-length span, got %s", result)
	}
}

func TestPseudonymiseOperatorAnonymiseSubstringPreservesContext(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := "test@example.com"
	start := 5
	end := 10 // "@exam"

	result := op.Anonymise(value, start, end, EntityEmailAddress)
	expected := "test" + result + "ple.com"

	if !strings.HasPrefix(expected, "test") || !strings.HasSuffix(expected, "ple.com") {
		t.Fatalf("Expected context preservation, got %s", expected)
	}
	// The middle part should be a pii token
	if !strings.Contains(expected, "pii:") {
		t.Fatalf("Expected pii token in result, got %s", expected)
	}
}

func TestPseudonymiseOperatorAnonymiseZeroLengthSpan(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := "test@example.com"
	result := op.Anonymise(value, 5, 5, EntityEmailAddress) // zero-length

	if result != value {
		t.Fatalf("Expected unchanged value for zero-length span, got %s", result)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	op1 := &PseudonymiseOperator{Key: []byte("00000000000000000000000000000000")} // 32 bytes
	op2 := &PseudonymiseOperator{Key: []byte("11111111111111111111111111111111")} // different key
	value := "test@example.com"

	encrypted := op1.Anonymise(value, 0, len(value), EntityEmailAddress)
	_, err := op2.Decrypt(encrypted)

	if err == nil {
		t.Fatalf("Expected decryption to fail with wrong key")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := "test@example.com"

	encrypted := op.Anonymise(value, 0, len(value), EntityEmailAddress)
	// Tamper with the ciphertext (change last character)
	tampered := encrypted[:len(encrypted)-1] + "x"

	_, err := op.Decrypt(tampered)
	if err == nil {
		t.Fatalf("Expected decryption to fail with tampered ciphertext")
	}
}

func TestDecryptMissingPrefix(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := hex.EncodeToString([]byte("ciphertext")) // just hex, no pii: prefix

	_, err := op.Decrypt(value)
	if err == nil {
		t.Fatalf("Expected decryption to fail with missing pii: prefix")
	}
}

func TestDecryptInvalidHex(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte("0123456789abcdef0123456789abcdef")}
	value := "pii:xyz" // invalid hex

	_, err := op.Decrypt(value)
	if err == nil {
		t.Fatalf("Expected decryption to fail with invalid hex")
	}
}

func TestDecryptEmptyKey(t *testing.T) {
	op := &PseudonymiseOperator{Key: []byte{}}
	value := "pii:" + hex.EncodeToString([]byte("ciphertext"))

	_, err := op.Decrypt(value)
	if err == nil {
		t.Fatalf("Expected decryption to fail with empty key")
	}
}

func TestCrossUserIsolation(t *testing.T) {
	// User A
	keyA := []byte("00000000000000000000000000000000")
	opA := &PseudonymiseOperator{Key: keyA}
	value := "test@example.com"
	encryptedA := opA.Anonymise(value, 0, len(value), EntityEmailAddress)

	// User B
	keyB := []byte("11111111111111111111111111111111")
	opB := &PseudonymiseOperator{Key: keyB}

	// User B cannot decrypt User A's token
	_, err := opB.Decrypt(encryptedA)
	if err == nil {
		t.Fatalf("Expected cross-user decryption to fail")
	}

	// User A can decrypt their own token
	plaintext, err := opA.Decrypt(encryptedA)
	if err != nil {
		t.Fatalf("Expected user A to decrypt their own token: %v", err)
	}
	if plaintext != value {
		t.Fatalf("Expected original value, got %s", plaintext)
	}
}
