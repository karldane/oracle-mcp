package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestCreditCardRecognizer_Valid(t *testing.T) {
	r := NewCreditCardRecognizer()
	cases := map[string]int{
		"4012888888881881":    1,
		"4012-8888-8888-1881": 1,
		"371449635398431":     1,
		"5555555555554444":    1,
		"6011000400000000":    1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityCreditCard, results[0].EntityType)
			}
		})
	}
}

func TestCreditCardRecognizer_Invalid(t *testing.T) {
	r := NewCreditCardRecognizer()
	cases := []string{
		"1234567890123456",
		"0000000000000000",
		"not a card",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestCreditCardRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewCreditCardRecognizer()
	assert.Equal(t, "CreditCardRecognizer", NewCreditCardRecognizer().Name())
}
