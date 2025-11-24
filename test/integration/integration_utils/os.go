package integrationutils

import (
	"os"
	"strings"
	"testing"
)

func CheckAllPluginsMissingFiles(t *testing.T) bool {
	t.Helper()

	return MissingFiles(t,
		[]string{
			PKIUAAConfigPath,
			PKIServiceConfigPath,
			NotificationUAAConfigPath,
			NotificationEndpointsPath,
			IdentityManagementConfigPath,
		},
	)
}

func MissingFiles(t *testing.T, requiredFiles []string) bool {
	t.Helper()

	for _, filePath := range requiredFiles {
		sFile, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			t.Skipf("Required config file not found: %s", filePath)
			return true
		}

		bPath := strings.ReplaceAll(filePath, "secret", "blueprints")

		bFile, err := os.Stat(bPath)
		if os.IsNotExist(err) {
			t.Logf("Missing blueprint file can not compare: %s ", bPath)
		} else if sFile.Size() == bFile.Size() {
			t.Skipf("Please update from blueprints values: %s", filePath)
			return true
		}
	}

	return false
}
