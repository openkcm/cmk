package builtin

import (
	"github.com/openkcm/cmk/internal/plugins/builtin/certificate_issuer"
	"github.com/openkcm/cmk/internal/plugins/builtin/identity_management"
	keystoreman "github.com/openkcm/cmk/internal/plugins/builtin/keystore/management"
	keystoreop "github.com/openkcm/cmk/internal/plugins/builtin/keystore/operations"
	"github.com/openkcm/cmk/internal/plugins/builtin/notification"
	"github.com/openkcm/cmk/internal/plugins/builtin/systeminformation"
	"github.com/openkcm/plugin-sdk/pkg/catalog"
)

func BuiltIns() []catalog.BuiltIn {
	return []catalog.BuiltIn{
		systeminformation.V1BuiltIn(),
		certificate_issuer.V1BuiltIn(),
		identity_management.V1BuiltIn(),
		keystoreop.V1BuiltIn(),
		keystoreman.V1BuiltIn(),
		notification.V1BuiltIn(),
	}
}
