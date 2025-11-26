package importparams

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

func ToAPI(p model.ImportParams) (*cmkapi.ImportParams, error) {
	err := sanitise.Stringlikes(&p)
	if err != nil {
		return nil, err
	}

	return &cmkapi.ImportParams{
		PublicKey: &p.PublicKeyPEM,
		WrappingAlgorithm: &cmkapi.WrappingAlgorithm{
			Name:         cmkapi.WrappingAlgorithmName(p.WrappingAlg),
			HashFunction: cmkapi.WrappingAlgorithmHashFunction(p.HashFunction),
		},
	}, nil
}
