package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestPassportRecognizer_Valid(t *testing.T) {
	r := NewPassportRecognizer()
	cases := map[string]int{
		"A12345678":  1,
		"912803456":  1,
		"AB12345678": 1,
		"123456789":  1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityPassportNumber, results[0].EntityType)
				assert.Greater(t, results[0].Score, 0.0)
			}
		})
	}
}

func TestPassportRecognizer_Invalid(t *testing.T) {
	r := NewPassportRecognizer()
	cases := []string{
		"not a passport",
		"123",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestPassportRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewPassportRecognizer()
	assert.Equal(t, "PassportRecognizer", NewPassportRecognizer().Name())
}
