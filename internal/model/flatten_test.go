package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type structNestedAndMaps struct {
	LocalityID  string            `flatten:"locality_id"`
	Nested      nested            `flatten:"nested"`
	MultiNested multinested       `flatten:"multi_nested"`
	Data        map[string]string `flatten:"data"`
}

func (w *structNestedAndMaps) FlattenRoot() string { return "root" }

type multinested struct {
	Nested nested `flatten:"nested"`
}

type nested struct {
	CommonName string `flatten:"common_name"`
}

type typedSample struct {
	Count   int    `flatten:"count"`
	Enabled bool   `flatten:"enabled"`
	Name    string `flatten:"name"`
}

func (w *typedSample) FlattenRoot() string { return "root" }

// withSkip checks the "-" tag and unexported fields are ignored.
type withSkip struct {
	Keep       string `flatten:"keep"`
	Skip       string `flatten:"-"`
	unexported string
}

func (w *withSkip) FlattenRoot() string { return "root" }

// withPointer checks nil and non-nil pointer fields.
type withPointer struct {
	Ptr *nested `flatten:"ptr"`
	Val string  `flatten:"val"`
}

func (w *withPointer) FlattenRoot() string { return "root" }

// emptyRoot checks FlattenRoot() == "" yields unprefixed keys.
type emptyRoot struct {
	Name string `flatten:"name"`
}

func (w *emptyRoot) FlattenRoot() string { return "" }

type withSlice struct {
	Items []string `flatten:"items"`
}

func (w *withSlice) FlattenRoot() string { return "root" }

type missingTag struct {
	Name string `flatten:"name"`
	Oops string
}

func (w *missingTag) FlattenRoot() string { return "root" }

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		in   Flattable
		want map[string]any // expected flat output; nil to skip the flat assertion
		out  Flattable      // pointer target for the round trip; nil to skip it
	}{
		{
			name: "nested structs and map",
			in: &structNestedAndMaps{
				LocalityID:  "loc-1",
				Nested:      nested{CommonName: "cn-1"},
				MultiNested: multinested{nested{CommonName: "cn-2"}},
				Data:        map[string]string{"a": "1", "b": "2"},
			},
			want: map[string]any{
				"root/locality_id":                     "loc-1",
				"root/nested/common_name":              "cn-1",
				"root/multi_nested/nested/common_name": "cn-2",
				"root/data/a":                          "1",
				"root/data/b":                          "2",
			},
			out: &structNestedAndMaps{},
		},
		{
			name: "preserves non-string types",
			in:   &typedSample{Count: 3, Enabled: true, Name: "x"},
			want: map[string]any{
				"root/count":   3,
				"root/enabled": true,
				"root/name":    "x",
			},
			out: &typedSample{},
		},
		{
			// Skip/unexported are dropped, so this case has no round trip.
			name: "skips dash tag and unexported",
			in:   &withSkip{Keep: "yes", Skip: "no", unexported: "hidden"},
			want: map[string]any{"root/keep": "yes"},
		},
		{
			name: "nil pointer skipped",
			in:   &withPointer{Val: "v"},
			want: map[string]any{"root/val": "v"},
			out:  &withPointer{},
		},
		{
			name: "non-nil pointer",
			in:   &withPointer{Ptr: &nested{CommonName: "cn"}, Val: "v"},
			want: map[string]any{"root/ptr/common_name": "cn", "root/val": "v"},
			out:  &withPointer{},
		},
		{
			name: "empty root yields unprefixed keys",
			in:   &emptyRoot{Name: "x"},
			want: map[string]any{"name": "x"},
			out:  &emptyRoot{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := Flatten(tt.in)
			require.NoError(t, err)
			if tt.want != nil {
				assert.Equal(t, tt.want, flat)
			}
			if tt.out != nil {
				require.NoError(t, Unflatten(flat, tt.out))
				assert.Equal(t, tt.in, tt.out)
			}
		})
	}
}

func TestFlattenErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      Flattable
		wantErr error
	}{
		{"slice unsupported", &withSlice{Items: []string{"a"}}, ErrUnsuportedField},
		{"missing tag", &missingTag{Name: "n", Oops: "x"}, ErrMissingTag},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Flatten(tt.in)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestUnflattenIgnoresUnknownKeys(t *testing.T) {
	flat := map[string]any{
		"root/locality_id": "loc-1",
		"root/unknown/x":   "ignored",
		"other/foo":        "foreign",
	}

	var got structNestedAndMaps
	require.NoError(t, Unflatten(flat, &got))
	assert.Equal(t, "loc-1", got.LocalityID)
}
