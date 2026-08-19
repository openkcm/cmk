package cmkapi_test

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/utils/enums"
)

// validator is implemented by every cmkapi enum type used here.
type validator interface {
	~string
	Valid() bool
}

// valuer is the driver.Valuer interface our enum_db.go Value() methods implement.
type valuer interface {
	Value() (driver.Value, error)
}

// runEnumValueTests asserts the standard Value() contract for an enum type.
func runEnumValueTests[T validator](t *testing.T, name string, validVal T, invalidErr error) {
	t.Helper()
	t.Run(name+"/valid", func(t *testing.T) {
		v, ok := any(validVal).(valuer)
		require.True(t, ok, "type does not implement driver.Valuer")
		got, err := v.Value()
		require.NoError(t, err)
		assert.Equal(t, string(validVal), got)
	})
	t.Run(name+"/empty becomes NULL", func(t *testing.T) {
		var zero T
		v, ok := any(zero).(valuer)
		require.True(t, ok, "type does not implement driver.Valuer")
		got, err := v.Value()
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run(name+"/invalid", func(t *testing.T) {
		v, ok := any(T("BOGUS")).(valuer)
		require.True(t, ok, "type does not implement driver.Valuer")
		_, err := v.Value()
		assert.ErrorIs(t, err, invalidErr)
	})
}

func TestKeyState_Valid(t *testing.T) {
	assert.True(t, cmkapi.KeyStateENABLED.Valid())
	assert.False(t, cmkapi.KeyState("").Valid())
	assert.False(t, cmkapi.KeyState("BOGUS").Valid())
}

func TestKeyState_Value(t *testing.T) {
	runEnumValueTests(t, "KeyState", cmkapi.KeyStateENABLED, cmkapi.ErrInvalidKeyState)
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
	runEnumValueTests(t, "SystemStatus", cmkapi.SystemStatusCONNECTED, cmkapi.ErrInvalidSystemStatus)
}

func TestSystemStatus_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     any
		want    cmkapi.SystemStatus
		wantErr error
	}{
		{name: "string", src: "CONNECTED", want: cmkapi.SystemStatusCONNECTED},
		{name: "bytes", src: []byte("FAILED"), want: cmkapi.SystemStatusFAILED},
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

func TestKeyType_Value(t *testing.T) {
	runEnumValueTests(t, "KeyType", cmkapi.KeyTypeBYOK, cmkapi.ErrInvalidKeyType)
}

func TestKeyType_Scan(t *testing.T) {
	var k cmkapi.KeyType
	require.NoError(t, k.Scan("HYOK"))
	assert.Equal(t, cmkapi.KeyTypeHYOK, k)
	assert.ErrorIs(t, k.Scan("BOGUS"), cmkapi.ErrInvalidKeyType)
}

func TestKeyAlgorithm_Value(t *testing.T) {
	runEnumValueTests(t, "KeyAlgorithm", cmkapi.KeyAlgorithmAES256, cmkapi.ErrInvalidKeyAlgorithm)
}

func TestKeyAlgorithm_Scan(t *testing.T) {
	var a cmkapi.KeyAlgorithm
	require.NoError(t, a.Scan("AES256"))
	assert.Equal(t, cmkapi.KeyAlgorithmAES256, a)
	assert.ErrorIs(t, a.Scan("BOGUS"), cmkapi.ErrInvalidKeyAlgorithm)
}

func TestWrappingAlgorithmName_Value(t *testing.T) {
	runEnumValueTests(t, "WrappingAlgorithmName",
		cmkapi.WrappingAlgorithmNameCKMRSAAESKEYWRAP, cmkapi.ErrInvalidWrappingAlgorithmName)
}

func TestWrappingAlgorithmName_Scan(t *testing.T) {
	var a cmkapi.WrappingAlgorithmName
	require.NoError(t, a.Scan("CKM_RSA_PKCS_OAEP"))
	assert.Equal(t, cmkapi.WrappingAlgorithmNameCKMRSAPKCSOAEP, a)
	assert.ErrorIs(t, a.Scan("BOGUS"), cmkapi.ErrInvalidWrappingAlgorithmName)
}

func TestWrappingAlgorithmHashFunction_Value(t *testing.T) {
	runEnumValueTests(t, "WrappingAlgorithmHashFunction",
		cmkapi.WrappingAlgorithmHashFunctionSHA256, cmkapi.ErrInvalidWrappingAlgorithmHashFunction)
}

func TestWrappingAlgorithmHashFunction_Scan(t *testing.T) {
	var h cmkapi.WrappingAlgorithmHashFunction
	require.NoError(t, h.Scan("SHA1"))
	assert.Equal(t, cmkapi.WrappingAlgorithmHashFunctionSHA1, h)
	assert.ErrorIs(t, h.Scan("BOGUS"), cmkapi.ErrInvalidWrappingAlgorithmHashFunction)
}
