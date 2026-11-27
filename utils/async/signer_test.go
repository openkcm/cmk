package async_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	asyncUtils "github.com/openkcm/cmk/utils/async"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

// tamperEnvelopePayload replaces the inner payload of a signed envelope while
// keeping the original signature, mimicking an attacker who cannot recompute the
// HMAC.
func tamperEnvelopePayload(t *testing.T, signed []byte, replacement asyncUtils.TaskPayload) []byte {
	t.Helper()

	var env map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(signed, &env))

	inner, err := replacement.ToBytes()
	require.NoError(t, err)

	encodedInner, err := json.Marshal(inner)
	require.NoError(t, err)
	env["payload"] = encodedInner

	out, err := json.Marshal(env)
	require.NoError(t, err)
	return out
}

func TestSigner_SignVerifyRoundTrip(t *testing.T) {
	signer := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, true)

	payload := asyncUtils.TaskPayload{TenantID: "t1", Data: []byte("abc")}
	signed, err := payload.Sign(signer)
	require.NoError(t, err)

	got, err := asyncUtils.ParseAndVerifyTaskPayload(context.Background(), signer, signed)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.TenantID)
	assert.Equal(t, []byte("abc"), got.Data)
}

func TestSigner_TamperedPayloadRejectedWhenEnforced(t *testing.T) {
	signer := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, true)

	signed, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).Sign(signer)
	require.NoError(t, err)

	tampered := tamperEnvelopePayload(t, signed, asyncUtils.TaskPayload{TenantID: "attacker"})

	_, err = asyncUtils.ParseAndVerifyTaskPayload(context.Background(), signer, tampered)
	assert.ErrorIs(t, err, asyncUtils.ErrInvalidSignature)
}

func TestSigner_TamperedPayloadAllowedWhenNotEnforced(t *testing.T) {
	signer := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, false)

	signed, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).Sign(signer)
	require.NoError(t, err)

	tampered := tamperEnvelopePayload(t, signed, asyncUtils.TaskPayload{TenantID: "attacker"})

	got, err := asyncUtils.ParseAndVerifyTaskPayload(context.Background(), signer, tampered)
	require.NoError(t, err)
	assert.Equal(t, "attacker", got.TenantID)
}

func TestSigner_UnsignedPayloadRejectedWhenEnforced(t *testing.T) {
	signer := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, true)

	raw, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).ToBytes()
	require.NoError(t, err)

	_, err = asyncUtils.ParseAndVerifyTaskPayload(context.Background(), signer, raw)
	assert.ErrorIs(t, err, asyncUtils.ErrMissingSignature)
}

func TestSigner_UnsignedPayloadAllowedWhenNotEnforced(t *testing.T) {
	signer := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, false)

	raw, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).ToBytes()
	require.NoError(t, err)

	got, err := asyncUtils.ParseAndVerifyTaskPayload(context.Background(), signer, raw)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.TenantID)
}

func TestSigner_RotationVerifiesPreviousKey(t *testing.T) {
	oldKey := randomKey(t)
	old := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: oldKey}, nil, true)
	signed, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).Sign(old)
	require.NoError(t, err)

	rotated := asyncUtils.NewSigner(
		asyncUtils.SigningKey{ID: "k2", Key: randomKey(t)},
		&asyncUtils.SigningKey{ID: "k1", Key: oldKey},
		true,
	)

	got, err := asyncUtils.ParseAndVerifyTaskPayload(context.Background(), rotated, signed)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.TenantID)
}

func TestSigner_UnknownKeyIDRejectedWhenEnforced(t *testing.T) {
	old := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k1", Key: randomKey(t)}, nil, true)
	signed, err := (&asyncUtils.TaskPayload{TenantID: "t1"}).Sign(old)
	require.NoError(t, err)

	other := asyncUtils.NewSigner(asyncUtils.SigningKey{ID: "k2", Key: randomKey(t)}, nil, true)

	_, err = asyncUtils.ParseAndVerifyTaskPayload(context.Background(), other, signed)
	assert.ErrorIs(t, err, asyncUtils.ErrUnknownSigningKey)
}

func TestSigner_NilSignerIsNoOp(t *testing.T) {
	payload := asyncUtils.TaskPayload{TenantID: "t1", Data: []byte("abc")}

	signed, err := payload.Sign(nil)
	require.NoError(t, err)

	raw, err := payload.ToBytes()
	require.NoError(t, err)
	assert.Equal(t, raw, signed)

	got, err := asyncUtils.ParseAndVerifyTaskPayload(context.Background(), nil, signed)
	require.NoError(t, err)
	assert.Equal(t, "t1", got.TenantID)
}
