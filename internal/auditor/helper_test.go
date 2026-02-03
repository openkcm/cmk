package auditor_test

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
)

var errLogsNotFound = errors.New("logs not found")

func createTestAuditor(endpoint string) *auditor.Auditor {
	cfg := config.Config{
		BaseConfig: commoncfg.BaseConfig{Audit: commoncfg.Audit{Endpoint: endpoint}},
	}

	return auditor.New(context.Background(), &cfg)
}

func generateTestCmkID() string {
	return uuid.New().String()
}

func getEventName(logs *plog.Logs) (string, error) {
	err := checkLogsExist(logs)
	if err != nil {
		return "", err
	}

	return logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName(), nil
}

func getAttributes(logs *plog.Logs) (map[string]string, error) {
	err := checkLogsExist(logs)
	if err != nil {
		return nil, err
	}

	attrs := make(map[string]string)

	recordAttrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	for k, v := range recordAttrs.All() {
		attrs[k] = v.AsString()
	}

	return attrs, nil
}

func checkLogsExist(logs *plog.Logs) error {
	if logs.ResourceLogs().Len() == 0 ||
		logs.ResourceLogs().At(0).ScopeLogs().Len() == 0 ||
		logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len() == 0 {
		return errLogsNotFound
	}

	return nil
}
