package presidio

type RecognizerRegistry struct {
	recognizers map[string]Recognizer
	entityIndex map[EntityType][]string
}

func NewRecognizerRegistry() *RecognizerRegistry {
	return &RecognizerRegistry{
		recognizers: make(map[string]Recognizer),
		entityIndex: make(map[EntityType][]string),
	}
}

func (r *RecognizerRegistry) Add(rec Recognizer) {
	name := rec.Name()
	r.recognizers[name] = rec
	for _, entity := range rec.SupportedEntities() {
		r.entityIndex[entity] = append(r.entityIndex[entity], name)
	}
}

func (r *RecognizerRegistry) Remove(name string) {
	if rec, ok := r.recognizers[name]; ok {
		for _, entity := range rec.SupportedEntities() {
			names := r.entityIndex[entity]
			for i, n := range names {
				if n == name {
					r.entityIndex[entity] = append(names[:i], names[i+1:]...)
					break
				}
			}
		}
		delete(r.recognizers, name)
	}
}

func (r *RecognizerRegistry) GetAll() []Recognizer {
	result := make([]Recognizer, 0, len(r.recognizers))
	for _, rec := range r.recognizers {
		result = append(result, rec)
	}
	return result
}

func (r *RecognizerRegistry) GetForEntities(entities []EntityType) []Recognizer {
	seen := make(map[string]bool)
	var result []Recognizer
	for _, entity := range entities {
		for _, name := range r.entityIndex[entity] {
			if !seen[name] {
				seen[name] = true
				result = append(result, r.recognizers[name])
			}
		}
	}
	return result
}
