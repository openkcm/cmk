package importparams

import (
	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/sanitise"
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
