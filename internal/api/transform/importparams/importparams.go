package importparams

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
)

func ToAPI(p model.ImportParams) *cmkapi.ImportParams {
	return &cmkapi.ImportParams{
		PublicKey: &p.PublicKeyPEM,
		WrappingAlgorithm: &cmkapi.WrappingAlgorithm{
			Name:         cmkapi.WrappingAlgorithmName(p.WrappingAlg),
			HashFunction: cmkapi.WrappingAlgorithmHashFunction(p.HashFunction),
		},
	}
}
