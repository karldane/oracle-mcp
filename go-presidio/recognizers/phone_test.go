package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestPhoneRecognizer_Valid(t *testing.T) {
	r := NewPhoneRecognizer()
	cases := map[string]int{
		"+44 7700 900123": 1,
		"(415) 555-0132":  1,
		"+14155550132":    1,
		"07700900123":     1,
		"02079461234":     1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityPhoneNumber, results[0].EntityType)
			}
		})
	}
}

func TestPhoneRecognizer_Invalid(t *testing.T) {
	r := NewPhoneRecognizer()
	cases := []string{
		"not a phone number",
		"12345",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestPhoneRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewPhoneRecognizer()
	assert.Equal(t, "PhoneRecognizer", NewPhoneRecognizer().Name())
}
