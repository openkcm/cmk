package errs

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/utils/ptr"
)

type Error[T any] interface {
	SetContext(m *map[string]any)
	DefaultError() T
}

type ErrorMapper[T Error[T]] struct {
	Errors         []ExposedErrors[T] // Error to mapTo
	PriorityErrors []ExposedErrors[T]
}

type ExposedErrors[T Error[T]] struct {
	InternalErrorChain []error                    // Errors to match on the mapper
	ExposedError       T                          // Can be ApiError, OrbitalError, etc.
	ContextGetter      func(error) map[string]any // Provide context from where the error was obtained
}

func NewMapper[T Error[T]](errors []ExposedErrors[T], priorityErrors []ExposedErrors[T]) ErrorMapper[T] {
	return ErrorMapper[T]{
		Errors:         errors,
		PriorityErrors: priorityErrors,
	}
}

// Transform has the following rules to find the best match:
// 1. If error is in priority return the priority one
// 2. Return ExposedError containing the highest number of errors in err chain
// 3. If no error found return default
func (m *ErrorMapper[T]) Transform(ctx context.Context, internalErr error) T {
	err, ok := m.containsAsPriority(internalErr)
	if ok {
		return err
	}
	result := m.getBestMatches(internalErr)

	if len(result) == 0 {
		err = *new(T)
		return err.DefaultError()
	}

	selected := result[0]

	err = selected.ExposedError
	if selected.ContextGetter != nil {
		err.SetContext(ptr.PointTo(selected.ContextGetter(internalErr)))
	}
	return err
}

// Checks if the err is in the PriorityErrors group
// If true, return the ExposedError
func (m *ErrorMapper[T]) containsAsPriority(err error) (T, bool) {
	for _, priorityErrors := range m.PriorityErrors {
		if countMatchingErrors(err, priorityErrors.InternalErrorChain) > 0 {
			return priorityErrors.ExposedError, true
		}
	}

	return *new(T), false
}

// Gets the APIErrors with the highest amount of matching errors as the err target chain
func (m *ErrorMapper[T]) getBestMatches(err error) []ExposedErrors[T] {
	minCount := 1

	var result []ExposedErrors[T]

	for _, mErr := range m.Errors {
		count := countMatchingErrors(err, mErr.InternalErrorChain)

		// Skip if mapping error contains errors that are not in the err
		if len(mErr.InternalErrorChain) > count {
			continue
		}

		if count == minCount {
			result = append(result, mErr)
		} else if count > minCount {
			minCount = count
			result = []ExposedErrors[T]{mErr}
		}
	}

	return result
}

// countMatchingErrors counts the number of errors in candidates that match err
func countMatchingErrors(err error, candidates []error) int {
	matchCount := 0

	for _, candidateErr := range candidates {
		if errors.Is(err, candidateErr) {
			matchCount++
		}
	}

	return matchCount
}
