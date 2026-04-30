package recognizers

import (
	"github.com/karldane/go-presidio/presidio"
)

// PersonRecognizer is a STUB implementation that returns no results.
// Per the go-presidio design decision, PERSON entity detection is deferred
// to a future phase that will leverage NLP/NER capabilities rather
// than pattern-based recognition.
type PersonRecognizer struct{}

func NewPersonRecognizer() *PersonRecognizer {
	return &PersonRecognizer{}
}

func (r *PersonRecognizer) Name() string { return "PersonRecognizer" }
func (r *PersonRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityPerson}
}

func (r *PersonRecognizer) Analyse(value string) []presidio.RecognizerResult {
	return nil
}
