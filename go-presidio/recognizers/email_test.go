package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestEmailRecognizer_ValidEmails(t *testing.T) {
	r := NewEmailRecognizer()
	cases := []struct {
		name     string
		input    string
		wantHits int
	}{
		{"simple", "info@presidio.site", 1},
		{"with text", "my email address is info@presidio.site", 1},
		{"two emails", "try info@presidio.site or anotherinfo@presidio.site", 2},
		{"subdomain", "test@mail.example.com", 1},
		{"plus addressing", "user+tag@example.org", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			results := r.Analyse(c.input)
			assert.Equal(t, c.wantHits, len(results), "input: %s", c.input)
			if c.wantHits > 0 {
				assert.Equal(t, presidio.EntityEmailAddress, results[0].EntityType)
				assert.GreaterOrEqual(t, results[0].Score, 0.8)
			}
		})
	}
}

func TestEmailRecognizer_InvalidEmails(t *testing.T) {
	r := NewEmailRecognizer()
	cases := []string{
		"info@presidio.",
		"info@.com",
		"@example.com",
		"plaintext",
		"missing@",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results, "should not detect: %s", input)
		})
	}
}

func TestEmailRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewEmailRecognizer()
	r := NewEmailRecognizer()
	assert.Equal(t, "EmailRecognizer", r.Name())
	assert.Contains(t, r.SupportedEntities(), presidio.EntityEmailAddress)
}
