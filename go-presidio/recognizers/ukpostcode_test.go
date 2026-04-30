package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestUkPostcodeRecognizer_Valid(t *testing.T) {
	r := NewUkPostcodeRecognizer()
	cases := map[string]int{
		"SW1A 1AA": 1,
		"M11AD":    1,
		"EC1A 1BB": 1,
		"BT1 1AB":  1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityUkPostcode, results[0].EntityType)
			}
		})
	}
}

func TestUkPostcodeRecognizer_Invalid(t *testing.T) {
	r := NewUkPostcodeRecognizer()
	cases := []string{
		"not a postcode",
		"12345",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestUkPostcodeRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewUkPostcodeRecognizer()
	assert.Equal(t, "UkPostcodeRecognizer", NewUkPostcodeRecognizer().Name())
}
