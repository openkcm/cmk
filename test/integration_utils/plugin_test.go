package integrationutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	integrationutils "github.com/openkcm/cmk/test/integration_utils"
)

func TestPlugins(t *testing.T) {
	t.Run("Should create SIS", func(t *testing.T) {
		p := integrationutils.SISPlugin(t)
		assert.Equal(t, "SYSINFO", p.Name)
	})

	t.Run("Should create PKI", func(t *testing.T) {
		p := integrationutils.PKIPlugin(t)
		assert.Equal(t, "CERT_ISSUER", p.Name)
	})

	t.Run("Should create AWS", func(t *testing.T) {
		p := integrationutils.KeystorePlugin(t)
		assert.Equal(t, "AWS", p.Name)
	})

	t.Run("Should create Notifications", func(t *testing.T) {
		p := integrationutils.NotificationPlugin(t)
		assert.Equal(t, "NOTIFICATION", p.Name)
	})
}
