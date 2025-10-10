package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/utils/ptr"
	"github.com/openkcm/cmk/utils/slice"
)

// Errs defines the errors that can be returned by the provider
var (
	ErrCreateKeyFailed        = errors.New("create key failed")
	ErrCreateKeyVersionFailed = errors.New("key version creation failed")
	ErrRotateKeyFailed        = errors.New("rotate key failed")
	ErrKeyVersions            = errors.New("key has no previous keyVersions")
	ErrEnableKeyFailed        = errors.New("enabling key failed")
	ErrDisableKeyFailed       = errors.New("disabling key failed")
	ErrDeleteKeyFailed        = errors.New("deleting key failed")
)

// Provider is the implementation of the KMS provider
type Provider struct {
	nativeClient Client
}

// NewProvider creates a new instance of Provider
func NewProvider(client Client) *Provider {
	return &Provider{
		nativeClient: client,
	}
}

// CreateKey creates a new key.
func (p *Provider) CreateKey(
	ctx context.Context,
	input KeyInput,
) (*Key, error) {
	firstVersion := 1

	version, err := p.createKeyVersion(ctx, input, firstVersion)
	if err != nil {
		return nil, fmt.Errorf("%w %w", ErrCreateKeyFailed, err)
	}

	k := &Key{
		ID:          input.ID,
		Version:     firstVersion,
		KeyVersions: []KeyVersion{version},
		KeyType:     input.KeyType,
	}

	return k, err
}

// RotateKey rotates the current key version
func (p *Provider) RotateKey(ctx context.Context, key *Key) error {
	if len(key.KeyVersions) == 0 {
		return ErrKeyVersions
	}

	input := KeyInput{KeyType: key.KeyType, ID: key.ID}
	newVersion := slice.LastElement(key.KeyVersions).Version + 1

	keyVersion, err := p.createKeyVersion(ctx, input, newVersion)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRotateKeyFailed, err)
	}

	key.KeyVersions = append(key.KeyVersions, keyVersion)
	key.Version = newVersion

	return nil
}

// EnableKey enables current key version
func (p *Provider) EnableKey(ctx context.Context, key *Key) error {
	if len(key.KeyVersions) == 0 {
		return ErrKeyVersions
	}

	currentVersion := &key.KeyVersions[len(key.KeyVersions)-1]

	err := p.nativeClient.EnableKeyVersion(ctx, *currentVersion.ExternalID)
	currentVersion.UpdatedAt = ptr.PointTo(time.Now())

	if err != nil {
		currentVersion.State = ERROR
		return fmt.Errorf("%w: %w", ErrEnableKeyFailed, err)
	}

	currentVersion.State = ENABLED

	return nil
}

// DisableKey disables all versions of a key
func (p *Provider) DisableKey(ctx context.Context, key *Key) error {
	return p.invokeActionOnAllVersions(
		ctx,
		key,
		p.nativeClient.DisableKeyVersion,
		DISABLED,
		ErrDisableKeyFailed,
	)
}

// DeleteKey deletes all versions of a key.
// Takes into consideration manually deleted versions.
func (p *Provider) DeleteKey(
	ctx context.Context,
	key *Key,
	deleteKeyOptions DeleteOptions,
) error {
	action := func(ctx context.Context, externalID string) error {
		err := p.nativeClient.DeleteKeyVersion(ctx, externalID, deleteKeyOptions)
		if err != nil {
			var stateErr *InvalidStateError
			if !errors.As(err, &stateErr) {
				return errs.Wrap(ErrDeleteKeyVersionFailed, err)
			}

			log.Warn(ctx, "Key Version probably deleted before", slog.String("ExternalID", externalID))
		}

		return nil
	}

	return p.invokeActionOnAllVersions(ctx, key, action, DELETED, ErrDeleteKeyFailed)
}

// createKeyVersion - creates a new version of the key
func (p *Provider) createKeyVersion(
	ctx context.Context,
	input KeyInput,
	version int,
) (KeyVersion, error) {
	nativeKeyID, err := p.nativeClient.CreateKeyVersion(ctx, input)
	if err != nil {
		return KeyVersion{}, fmt.Errorf("%w: %w", ErrCreateKeyVersionFailed, err)
	}

	return KeyVersion{
		ExternalID: nativeKeyID,
		CreatedAt:  ptr.PointTo(time.Now()),
		Version:    version,
		State:      ENABLED,
	}, nil
}

// invokeActionOnAllVersions performs the same action on all versions of a key
func (p *Provider) invokeActionOnAllVersions(
	ctx context.Context,
	key *Key,
	action func(context.Context, string) error,
	successState KeyState,
	errorType error,
) error {
	for i := range key.KeyVersions {
		keyVersionError := action(ctx, *key.KeyVersions[i].ExternalID)
		key.KeyVersions[i].UpdatedAt = ptr.PointTo(time.Now())

		if keyVersionError != nil {
			key.KeyVersions[i].State = ERROR

			return fmt.Errorf("%w: %w", errorType, keyVersionError)
		}

		key.KeyVersions[i].State = successState
	}

	return nil
}
