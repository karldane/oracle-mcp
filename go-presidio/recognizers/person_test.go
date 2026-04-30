package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestPersonRecognizer_StubReturnsNil(t *testing.T) {
	r := NewPersonRecognizer()
	cases := []string{
		"John Doe",
		"Jane Smith",
		"Any person name",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Nil(t, results)
		})
	}
}

func TestPersonRecognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewPersonRecognizer()
	assert.Equal(t, "PersonRecognizer", NewPersonRecognizer().Name())
}
