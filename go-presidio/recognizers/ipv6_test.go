package recognizers

import (
	"testing"

	"github.com/karldane/go-presidio/presidio"
	"github.com/stretchr/testify/assert"
)

func TestIPv6Recognizer_Valid(t *testing.T) {
	r := NewIPv6Recognizer()
	cases := map[string]int{
		"2001:db8::1":                        1,
		"fe80::1":                            1,
		"::1":                                1,
		"2001:db8::/32":                      1,
		"fe80::/10":                          1,
		"2001:0db8:0000:0000:0000:0000:0001": 1,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Equal(t, want, len(results), "input: %s", input)
			if want > 0 {
				assert.Equal(t, presidio.EntityIPv6, results[0].EntityType)
			}
		})
	}
}

func TestIPv6Recognizer_Invalid(t *testing.T) {
	r := NewIPv6Recognizer()
	cases := []string{
		"not an ipv6",
		"gggg::1111",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			results := r.Analyse(input)
			assert.Empty(t, results, "should not detect: %s", input)
		})
	}
}

func TestIPv6Recognizer_Interface(t *testing.T) {
	var _ presidio.Recognizer = NewIPv6Recognizer()
	assert.Equal(t, "IPv6Recognizer", NewIPv6Recognizer().Name())
}
