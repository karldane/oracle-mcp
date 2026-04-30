package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestUkNinoRecognizer_Valid(t *testing.T) {
	r := NewUkNinoRecognizer()
	cases := map[string]int{
		"AA 12 34 56 B": 1,
		"hh 01 02 03 d": 1,
		"tw987654a":     1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityUkNino, results[0].EntityType)
			}
		})
	}
}

func TestUkNinoRecognizer_Invalid(t *testing.T) {
	r := NewUkNinoRecognizer()
	cases := []string{
		"AA 12 34 56 H",
		"FQ 00 00 00 C",
		"not a nino",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestUkNinoRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewUkNinoRecognizer()
	assert.Equal(t, "UkNinoRecognizer", NewUkNinoRecognizer().Name())
}
