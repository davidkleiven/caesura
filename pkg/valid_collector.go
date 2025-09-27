package pkg

// ValidCollector is a type that collects all elements that does not return an error
// when passed to DataTo
type ValidCollector[T any] struct {
	Items []T
	Err   error
}

func (v *ValidCollector[T]) Push(doc Document) {
	var obj T
	err := doc.DataTo(&obj)
	if err != nil {
		v.Err = err
		return
	}
	v.Items = append(v.Items, obj)
}

func NewValidCollector[T any]() *ValidCollector[T] {
	return &ValidCollector[T]{
		Items: []T{},
	}
}
