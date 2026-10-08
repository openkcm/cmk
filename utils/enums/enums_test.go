package enums_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/enums"
)

type colour string

const (
	red   colour = "RED"
	green colour = "GREEN"
	blue  colour = "BLUE"
)

var (
	allColours   = []colour{red, green, blue}
	errBadColour = errors.New("invalid colour")
)

func (c colour) Valid() bool { return slices.Contains(allColours, c) }

func TestValue(t *testing.T) {
	tests := []struct {
		name    string
		in      colour
		want    any
		wantErr error
	}{
		{name: "valid", in: red, want: "RED"},
		{name: "empty becomes NULL", in: "", want: nil},
		{name: "unknown rejected", in: colour("PUCE"), want: nil, wantErr: errBadColour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := enums.Value(tt.in, errBadColour)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScan(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var c colour
		assert.NoError(t, enums.Scan("GREEN", &c, errBadColour))
		assert.Equal(t, green, c)
	})

	t.Run("bytes", func(t *testing.T) {
		var c colour
		assert.NoError(t, enums.Scan([]byte("BLUE"), &c, errBadColour))
		assert.Equal(t, blue, c)
	})

	t.Run("nil clears to zero", func(t *testing.T) {
		c := red
		assert.NoError(t, enums.Scan(nil, &c, errBadColour))
		assert.Equal(t, colour(""), c)
	})

	t.Run("unknown value leaves out unchanged", func(t *testing.T) {
		c := red
		err := enums.Scan("PUCE", &c, errBadColour)
		assert.ErrorIs(t, err, errBadColour)
		assert.Equal(t, red, c, "Scan must not write to *out when validation fails")
	})

	t.Run("wrong type", func(t *testing.T) {
		var c colour
		err := enums.Scan(42, &c, errBadColour)
		assert.ErrorIs(t, err, enums.ErrUnexpectedScanType)
	})
}
