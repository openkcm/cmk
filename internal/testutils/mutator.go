package testutils

// NewMutator provides a function that can be used in table driven tests.
// It returns a function that can be used to mutate an object, where the object
// is provided by initTProv.
func NewMutator[T any](baseProv func() T) func(mutatorFn ...func(*T)) T {
	return func(m ...func(*T)) T {
		base := baseProv()
		for i := range m {
			m[i](&base)
		}

		return base
	}
}
