package providers

import (
	"context"
)

// ExportCreateKeyVersion is a function that returns a function to create a key version
func (p *Provider) ExportCreateKeyVersion() func(ctx context.Context, input KeyInput, version int) (KeyVersion, error) {
	return p.createKeyVersion
}
