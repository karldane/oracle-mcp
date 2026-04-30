package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestUsSsnRecognizer_Valid(t *testing.T) {
	r := NewUsSsnRecognizer()
	cases := map[string]int{
		"078-05-1123": 1,
		"123-45-6789": 1,
		"987654321":   1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityUsSsn, results[0].EntityType)
			}
		})
	}
}

func TestUsSsnRecognizer_Invalid(t *testing.T) {
	r := NewUsSsnRecognizer()
	cases := []string{
		"000-xx-xxxx",
		"666-xx-xxxx",
		"078-05-1120",
		"not a ssn",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestUsSsnRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewUsSsnRecognizer()
	assert.Equal(t, "UsSsnRecognizer", NewUsSsnRecognizer().Name())
}
