package presidio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

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
	key := LoadOperatorConfig().HMACKey
	if len(key) == 0 {
		return "[hash requires HMAC key]"
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

type MaskOperator struct {
	KeepLast int
}

func (o *MaskOperator) Name() string { return "mask" }
func (o *MaskOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if o.KeepLast <= 0 {
		o.KeepLast = 4
	}
	if len(value) <= o.KeepLast {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", len(value)-o.KeepLast) + value[len(value)-o.KeepLast:]
}

type PseudonymiseOperator struct{}

func (o *PseudonymiseOperator) Name() string { return "pseudonymise" }
func (o *PseudonymiseOperator) Anonymise(value string, start, end int, entity EntityType) string {
	key := LoadOperatorConfig().HMACKey
	if len(key) == 0 {
		return "[pseudonymise requires HMAC key]"
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	hash := hex.EncodeToString(h.Sum(nil))
	prefix := map[EntityType]string{
		EntityPerson:       "PERSON",
		EntityEmailAddress: "EMAIL",
		EntityPhoneNumber:  "PHONE",
	}
	if p, ok := prefix[entity]; ok {
		return fmt.Sprintf("%s_%s", p, hash[:8])
	}
	return hash[:12]
}

type ReplaceOperator struct {
	Replacement string
}

func (o *ReplaceOperator) Name() string { return "replace" }
func (o *ReplaceOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if o.Replacement == "" {
		o.Replacement = "[REPLACED]"
	}
	return o.Replacement
}

type CustomOperator struct {
	Fn func(value string, entity EntityType) string
}

func (o *CustomOperator) Name() string { return "custom" }
func (o *CustomOperator) Anonymise(value string, start, end int, entity EntityType) string {
	if o.Fn == nil {
		return value
	}
	return o.Fn(value, entity)
}
