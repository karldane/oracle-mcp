package presidio

type Recognizer interface {
	Name() string
	SupportedEntities() []EntityType
	Analyse(value string) []RecognizerResult
}
