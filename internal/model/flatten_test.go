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

func (w structNestedAndMaps) FlattenKeyRoot() string { return "root" }
func (w structNestedAndMaps) Type() string           { return "root" }

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

func (w typedSample) FlattenKeyRoot() string { return "root" }

// withSkip checks the "-" tag and unexported fields are ignored.
type withSkip struct {
	Keep       string `flatten:"keep"`
	Skip       string `flatten:"-"`
	unexported string
}

func (w withSkip) FlattenKeyRoot() string { return "root" }

// withPointer checks nil and non-nil pointer fields.
type withPointer struct {
	Ptr *nested `flatten:"ptr"`
	Val string  `flatten:"val"`
}

func (w withPointer) FlattenKeyRoot() string { return "root" }

// emptyRoot checks FlattenKeyRoot() == "" yields unprefixed keys.
type emptyRoot struct {
	Name string `flatten:"name"`
}

func (w emptyRoot) FlattenKeyRoot() string { return "" }

type withSlice struct {
	Items []string `flatten:"items"`
}

func (w withSlice) FlattenKeyRoot() string { return "root" }

type missingTag struct {
	Name string `flatten:"name"`
	Oops string
}

func (w missingTag) FlattenKeyRoot() string { return "root" }

func TestFlatten(t *testing.T) {
	tests := []struct {
		name      string
		in        Flattable
		want      map[string]any // expected flat output; nil to skip the flat assertion
		unflatten func(map[string]any) (Flattable, error)
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
			unflatten: func(flat map[string]any) (Flattable, error) {
				got, err := Unflatten[structNestedAndMaps](flat)
				return &got, err
			},
		},
		{
			name: "preserves non-string types",
			in:   &typedSample{Count: 3, Enabled: true, Name: "x"},
			want: map[string]any{
				"root/count":   3,
				"root/enabled": true,
				"root/name":    "x",
			},
			unflatten: func(flat map[string]any) (Flattable, error) {
				got, err := Unflatten[typedSample](flat)
				return &got, err
			},
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
			unflatten: func(flat map[string]any) (Flattable, error) {
				got, err := Unflatten[withPointer](flat)
				return &got, err
			},
		},
		{
			name: "non-nil pointer",
			in:   &withPointer{Ptr: &nested{CommonName: "cn"}, Val: "v"},
			want: map[string]any{"root/ptr/common_name": "cn", "root/val": "v"},
			unflatten: func(flat map[string]any) (Flattable, error) {
				got, err := Unflatten[withPointer](flat)
				return &got, err
			},
		},
		{
			name: "empty root yields unprefixed keys",
			in:   &emptyRoot{Name: "x"},
			want: map[string]any{"name": "x"},
			unflatten: func(flat map[string]any) (Flattable, error) {
				got, err := Unflatten[emptyRoot](flat)
				return &got, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := Flatten(tt.in)
			require.NoError(t, err)
			if tt.want != nil {
				assert.Equal(t, tt.want, flat)
			}
			if tt.unflatten != nil {
				got, err := tt.unflatten(flat)
				require.NoError(t, err)
				assert.Equal(t, tt.in, got)
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

	got, err := Unflatten[structNestedAndMaps](flat)
	require.NoError(t, err)
	assert.Equal(t, "loc-1", got.LocalityID)
}
