package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestDobRecognizer_Valid(t *testing.T) {
	r := NewDobRecognizer()
	cases := map[string]int{
		"05-MAY-2021": 1,
		"2021-05-21":  1,
		"5/20/2021":   1,
		"01/01/2000":  1,
		"2020-12-31":  1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityDateOfBirth, results[0].EntityType)
			}
		})
	}
}

func TestDobRecognizer_Invalid(t *testing.T) {
	r := NewDobRecognizer()
	cases := []string{
		"not a date",
		"13-55-2020", // Invalid month/day
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestDobRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewDobRecognizer()
	assert.Equal(t, "DobRecognizer", NewDobRecognizer().Name())
}
