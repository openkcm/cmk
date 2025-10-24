package importparams

import (
	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/model"
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
