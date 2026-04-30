package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestIbanRecognizer_Valid(t *testing.T) {
	r := NewIbanRecognizer()
	cases := map[string]int{
		"DE8937040044050513100000": 1,
		"GB82WEST12345698765432":   1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityIban, results[0].EntityType)
			}
		})
	}
}

func TestIbanRecognizer_Invalid(t *testing.T) {
	r := NewIbanRecognizer()
	cases := []string{
		"DE8937040044050513100001",
		"INVALID",
		"not an iban",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestIbanRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewIbanRecognizer()
	assert.Equal(t, "IbanRecognizer", NewIbanRecognizer().Name())
}
