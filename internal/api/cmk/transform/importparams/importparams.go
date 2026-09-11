package importparams

import (
	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

func ToAPI(p model.ImportParams) (*cmkapi.ImportParams, error) {
	err := sanitise.Sanitize(&p)
	if err != nil {
		return nil, err
	}

	return &cmkapi.ImportParams{
		PublicKey: &p.PublicKeyPEM,
		WrappingAlgorithm: &cmkapi.WrappingAlgorithm{
			Name:         p.WrappingAlg,
			HashFunction: p.HashFunction,
		},
	}, nil
}
