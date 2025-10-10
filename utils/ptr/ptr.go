package ptr

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

// PointTo creates a typed pointer of whatever you hand in as parameter
func PointTo[T any](t T) *T {
	return &t
}

// PanicIfDifferent - panics if arguments are of different type
func PanicIfDifferent[T any](current, expected T) {
	if reflect.ValueOf(current).Pointer() != reflect.ValueOf(expected).Pointer() {
		panic(
			fmt.Sprintf(
				"must be %v",
				runtime.FuncForPC(reflect.ValueOf(expected).Pointer()).Name(),
			),
		)
	}
}

func GetIntOrDefault(ptr *int, def int) int {
	if ptr == nil {
		return def
	}

	return *ptr
}

func IsValidStrPtr(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

func IsNotNilUUID(ptr *uuid.UUID) bool {
	return ptr != nil && *ptr != uuid.Nil
}

// GetSafeDeref returns the dereferenced value of a pointer or the zero value of T if the pointer is nil.
func GetSafeDeref[T any](ptr *T) T {
	var res T
	if ptr != nil {
		res = *ptr
	}

	return res
}

func PointerArrayToValueArray[T any](pointerArray []*T) []T {
	valueArray := make([]T, len(pointerArray))
	for i := range pointerArray {
		valueArray[i] = *pointerArray[i]
	}

	return valueArray
}

// InitializerFunc and Initializer to allow initialisation of a generic
type InitializerFunc[T any] func() T

func Initializer[T any]() *T {
	return new(T)
}
