package builtin

import (
	"github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk-core/internal/plugins/builtin/certificateissuer"
	"github.com/openkcm/cmk-core/internal/plugins/builtin/identitymanagement"
	keystoreman "github.com/openkcm/cmk-core/internal/plugins/builtin/keystore/management"
	keystoreop "github.com/openkcm/cmk-core/internal/plugins/builtin/keystore/operations"
	"github.com/openkcm/cmk-core/internal/plugins/builtin/notification"
	"github.com/openkcm/cmk-core/internal/plugins/builtin/systeminformation"
)

func BuiltIns() []catalog.BuiltIn {
	return []catalog.BuiltIn{
		systeminformation.V1BuiltIn(),
		certificateissuer.V1BuiltIn(),
		identitymanagement.V1BuiltIn(),
		keystoreop.V1BuiltIn(),
		keystoreman.V1BuiltIn(),
		notification.V1BuiltIn(),
	}
}
