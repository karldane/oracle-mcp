package presidio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	siv "github.com/secure-io/siv-go"
)

var piiPrefix = "pii:"

type Operator interface {
	Name() string
	Anonymise(value string, start, end int, entity EntityType) string
}

type OperatorConfig struct {
	Operators       map[EntityType]Operator
	DefaultOperator Operator
	HMACKey         []byte
}

func LoadOperatorConfig() *OperatorConfig {
	cfg := &OperatorConfig{
		Operators: make(map[EntityType]Operator),
	}
	if key := os.Getenv("PRESIDIO_HMAC_KEY"); key != "" {
		cfg.HMACKey = []byte(key)
	}
	cfg.DefaultOperator = &RedactOperator{}
	cfg.Operators[EntityEmailAddress] = cfg.DefaultOperator
	cfg.Operators[EntityPhoneNumber] = cfg.DefaultOperator
	cfg.Operators[EntityUkPostcode] = cfg.DefaultOperator
	cfg.Operators[EntityUkNino] = cfg.DefaultOperator
	cfg.Operators[EntityUkNhsNumber] = cfg.DefaultOperator
	cfg.Operators[EntityCreditCard] = cfg.DefaultOperator
	cfg.Operators[EntityIban] = cfg.DefaultOperator
	cfg.Operators[EntityIPv4] = cfg.DefaultOperator
	cfg.Operators[EntityIPv6] = cfg.DefaultOperator
	cfg.Operators[EntityDateOfBirth] = cfg.DefaultOperator
	cfg.Operators[EntityPerson] = cfg.DefaultOperator
	cfg.Operators[EntityUsSsn] = cfg.DefaultOperator
	cfg.Operators[EntityPassportNumber] = cfg.DefaultOperator
	cfg.Operators[EntityDriverLicence] = cfg.DefaultOperator
	return cfg
}

type RedactOperator struct{}

func (o *RedactOperator) Name() string { return "redact" }

func (o *RedactOperator) Anonymise(value string, start, end int, entity EntityType) string {
	return SentinelRedactedFor(entity)
}

type HashOperator struct{}

func (o *HashOperator) Name() string { return "hash" }

func (o *HashOperator) Anonymise(value string, start, end int, entity EntityType) string {
	key := []byte(os.Getenv("PRESIDIO_HMAC_KEY"))
	if len(key) == 0 {
		return "[hash requires HMAC key]"
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value[start:end]))
	return hex.EncodeToString(h.Sum(nil))
}

type MaskOperator struct {
	KeepLast int
}

func (o *MaskOperator) Name() string { return "mask" }

func (o *MaskOperator) Anonymise(value string, start, end int, entity EntityType) string {
	segment := value[start:end]
	if o.KeepLast <= 0 {
		o.KeepLast = 4
	}
	if len(segment) <= o.KeepLast {
		return strings.Repeat("*", len(segment))
	}
	return strings.Repeat("*", len(segment)-o.KeepLast) + segment[len(segment)-o.KeepLast:]
}

type PseudonymiseOperator struct {
	Key []byte
}

func (o *PseudonymiseOperator) Name() string { return "pseudonymise" }

func (o *PseudonymiseOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if end <= start {
		return value
	}
	if len(o.Key) == 0 {
		return "[pseudonymise requires key]"
	}
	aead, err := siv.NewCMAC(o.Key)
	if err != nil {
		return fmt.Sprintf("[pseudonymise error: encryption failed: %v]", err)
	}
	ciphertext := aead.Seal(nil, nil, []byte(value[start:end]), nil)
	token := piiPrefix + hex.EncodeToString(ciphertext)
	return value[:start] + token + value[end:]
}

func (o *PseudonymiseOperator) Decrypt(token string) (string, error) {
	if len(o.Key) == 0 {
		return "", fmt.Errorf("presidio: no key configured")
	}
	if !strings.HasPrefix(token, piiPrefix) {
		return "", fmt.Errorf("presidio: not a PII token: %q", token)
	}
	ciphertext, err := hex.DecodeString(token[len(piiPrefix):])
	if err != nil {
		return "", fmt.Errorf("presidio: invalid token encoding: %w", err)
	}
	aead, err := siv.NewCMAC(o.Key)
	if err != nil {
		return "", fmt.Errorf("presidio: failed to init cipher: %w", err)
	}
	plaintext, err := aead.Open(nil, nil, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("presidio: decryption failed (wrong key or tampered ciphertext)")
	}
	return string(plaintext), nil
}

type ReplaceOperator struct {
	Replacement string
}

func (o *ReplaceOperator) Name() string { return "replace" }

func (o *ReplaceOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if o.Replacement == "" {
		o.Replacement = "[REPLACED]"
	}
	return value[:start] + o.Replacement + value[end:]
}

type CustomOperator struct {
	Fn func(value string, entity EntityType) string
}

func (o *CustomOperator) Name() string { return "custom" }

func (o *CustomOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if o.Fn == nil {
		return value
	}
	return value[:start] + o.Fn(value[start:end], entity) + value[end:]
}
