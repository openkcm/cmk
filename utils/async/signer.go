package async

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
)

var (
	ErrMissingSignature  = errors.New("task payload is not signed")
	ErrInvalidSignature  = errors.New("task payload signature is invalid")
	ErrUnknownSigningKey = errors.New("task payload signed with an unknown key ID")
)

var signatureFailureCounter metric.Int64Counter

// defaultSigner is the process-wide signer, installed once at startup.
var (
	defaultSigner     atomic.Pointer[Signer]
	defaultSignerOnce sync.Once
)

// SetDefaultSigner installs the process-wide signer on the first call and ignores
// later calls, so a signer cannot be swapped out after startup.
func SetDefaultSigner(s *Signer) {
	defaultSignerOnce.Do(func() {
		defaultSigner.Store(s)
	})
}

// DefaultSigner returns the process-wide signer, nil until SetDefaultSigner runs.
func DefaultSigner() *Signer {
	return defaultSigner.Load()
}

func init() {
	counter, err := otel.Meter("github.com/openkcm/cmk/utils/async").Int64Counter(
		"cmk_async_task_signature_failures_total",
	)
	if err == nil {
		signatureFailureCounter = counter
	}
}

// recordSignatureFailure logs and counts a payload that failed verification, so a
// non-enforcing rollout is observable before it starts rejecting.
func recordSignatureFailure(ctx context.Context, err error) {
	log.Warn(ctx, "async task payload failed signature verification", log.ErrorAttr(err))
	if signatureFailureCounter != nil {
		signatureFailureCounter.Add(ctx, 1)
	}
}

type SigningKey struct {
	ID  string
	Key []byte
}

// Signer signs and verifies async task payloads with HMAC-SHA256.
// A Signer with no key signs and verifies as no-ops, so an unconfigured
// deployment keeps working. When not enforcing, Verify returns the payload
// alongside any signature error so a rollout can observe before it rejects.
type Signer struct {
	current  SigningKey
	previous *SigningKey
	enforce  bool
}

func NewSigner(current SigningKey, previous *SigningKey, enforce bool) *Signer {
	return &Signer{current: current, previous: previous, enforce: enforce}
}

func (s *Signer) Enforced() bool {
	return s.active() && s.enforce
}

type signedEnvelope struct {
	Payload   []byte `json:"payload"`
	KeyID     string `json:"keyID"`
	Signature []byte `json:"signature"`
}

func (s *Signer) Sign(payload []byte) ([]byte, error) {
	if !s.active() {
		return payload, nil
	}

	out, err := json.Marshal(signedEnvelope{
		Payload:   payload,
		KeyID:     s.current.ID,
		Signature: sign(s.current.Key, payload),
	})
	if err != nil {
		return nil, errs.Wrap(ErrParsingPayload, err)
	}

	return out, nil
}

// Verify unwraps a signed envelope and returns the inner payload. On a missing
// or bad signature it returns the payload with a non-nil error; a caller that is
// not enforcing may use the payload and only log the error.
func (s *Signer) Verify(data []byte) ([]byte, error) {
	if !s.active() {
		return data, nil
	}

	env, ok := decodeEnvelope(data)
	if !ok {
		return data, ErrMissingSignature
	}

	key, err := s.keyFor(env.KeyID)
	if err != nil {
		return env.Payload, err
	}

	if !hmac.Equal(env.Signature, sign(key, env.Payload)) {
		return env.Payload, ErrInvalidSignature
	}

	return env.Payload, nil
}

func (s *Signer) active() bool {
	return s != nil && len(s.current.Key) > 0
}

func (s *Signer) keyFor(keyID string) ([]byte, error) {
	if keyID == s.current.ID {
		return s.current.Key, nil
	}
	if s.previous != nil && keyID == s.previous.ID {
		return s.previous.Key, nil
	}
	return nil, ErrUnknownSigningKey
}

func sign(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return mac.Sum(nil)
}

// decodeEnvelope reports false for a legacy unsigned payload so it flows through
// the unsigned path during rollout.
func decodeEnvelope(data []byte) (signedEnvelope, bool) {
	var env signedEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return signedEnvelope{}, false
	}
	if len(env.Signature) == 0 || env.KeyID == "" {
		return signedEnvelope{}, false
	}
	return env, true
}
