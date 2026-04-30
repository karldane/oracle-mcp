package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestIPv4Recognizer_Valid(t *testing.T) {
	r := NewIPv4Recognizer()
	cases := map[string]int{
		"192.168.0.1":                       1,
		"10.0.0.1":                          1,
		"172.16.0.1":                        1,
		"192.168.1.1/24":                    1,
		"10.0.0.0/8":                        1,
		"multiple 10.0.0.1 and 192.168.1.1": 2,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityIPv4, results[0].EntityType)
			}
		})
	}
}

func TestIPv4Recognizer_Invalid(t *testing.T) {
	r := NewIPv4Recognizer()
	cases := []string{
		"256.0.0.1",
		"192.168.1.256",
		"not an ip",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results)
		})
	}
}

func TestIPv4Recognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewIPv4Recognizer()
	assert.Equal(t, "IPv4Recognizer", NewIPv4Recognizer().Name())
}
