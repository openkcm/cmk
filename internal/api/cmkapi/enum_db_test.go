package cmkapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/utils/enums"
)

func TestKeyState_Valid(t *testing.T) {
	assert.True(t, cmkapi.KeyStateENABLED.Valid())
	assert.False(t, cmkapi.KeyState("").Valid())
	assert.False(t, cmkapi.KeyState("BOGUS").Valid())
}

func TestKeyState_Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v, err := cmkapi.KeyStateENABLED.Value()
		assert.NoError(t, err)
		assert.Equal(t, "ENABLED", v)
	})

	t.Run("empty becomes NULL", func(t *testing.T) {
		v, err := cmkapi.KeyState("").Value()
		assert.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := cmkapi.KeyState("BOGUS").Value()
		assert.ErrorIs(t, err, cmkapi.ErrInvalidKeyState)
	})
}

func TestKeyState_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    cmkapi.KeyState
		wantErr error
	}{
		{name: "string", src: "ENABLED", want: cmkapi.KeyStateENABLED},
		{name: "bytes", src: []byte("DISABLED"), want: cmkapi.KeyStateDISABLED},
		{name: "nil clears", src: nil, want: cmkapi.KeyState("")},
		{name: "invalid", src: "BOGUS", wantErr: cmkapi.ErrInvalidKeyState},
		{name: "wrong type", src: 1, wantErr: enums.ErrUnexpectedScanType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s cmkapi.KeyState
			err := s.Scan(tt.src)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestSystemStatus_Valid(t *testing.T) {
	assert.True(t, cmkapi.SystemStatusCONNECTED.Valid())
	assert.False(t, cmkapi.SystemStatus("").Valid())
	assert.False(t, cmkapi.SystemStatus("BOGUS").Valid())
}

func TestSystemStatus_Value(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		v, err := cmkapi.SystemStatusCONNECTED.Value()
		assert.NoError(t, err)
		assert.Equal(t, "CONNECTED", v)
	})

	t.Run("empty becomes NULL", func(t *testing.T) {
		v, err := cmkapi.SystemStatus("").Value()
		assert.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := cmkapi.SystemStatus("BOGUS").Value()
		assert.ErrorIs(t, err, cmkapi.ErrInvalidSystemStatus)
	})
}

func TestSystemStatus_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    cmkapi.SystemStatus
		wantErr error
	}{
		{name: "string", src: "CONNECTED", want: cmkapi.SystemStatusCONNECTED},
		{name: "bytes", src: []byte("UNDER_WORKFLOW"), want: cmkapi.SystemStatusUNDERWORKFLOW},
		{name: "nil clears", src: nil, want: cmkapi.SystemStatus("")},
		{name: "invalid", src: "BOGUS", wantErr: cmkapi.ErrInvalidSystemStatus},
		{name: "wrong type", src: 1.5, wantErr: enums.ErrUnexpectedScanType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s cmkapi.SystemStatus
			err := s.Scan(tt.src)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, s)
		})
	}
}
