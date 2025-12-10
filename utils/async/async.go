package async

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/openkcm/common-sdk/pkg/auth"

	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	ctxUtils "github.tools.sap/kms/cmk/utils/context"
)

var (
	ErrParsingPayload = errors.New("could not parse task payload")
)

// TaskPayload represents the payload for an async task, including tenant context.
type TaskPayload struct {
	TenantID   string
	ClientData auth.ClientData
	Data       []byte
}

func NewTaskPayload(ctx context.Context, data []byte) TaskPayload {
	tenantID, err := ctxUtils.ExtractTenantID(ctx)
	if err != nil {
		tenantID = ""
	}

	clientData, err := ctxUtils.ExtractClientData(ctx)
	if err != nil {
		clientData = &auth.ClientData{}
	}

	return TaskPayload{
		TenantID:   tenantID,
		ClientData: *clientData,
		Data:       data,
	}
}

func ParseTaskPayload(payload []byte) (TaskPayload, error) {
	var p TaskPayload

	err := json.Unmarshal(payload, &p)
	if err != nil {
		return TaskPayload{}, errs.Wrap(ErrParsingPayload, err)
	}

	return p, nil
}

func (p *TaskPayload) InjectContext(ctx context.Context) context.Context {
	if p.TenantID != "" {
		ctx = ctxUtils.CreateTenantContext(ctx, p.TenantID)
	}

	return context.WithValue(ctx, constants.ClientData, &p.ClientData)
}

func (p *TaskPayload) ToBytes() ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, errs.Wrap(ErrParsingPayload, err)
	}

	return data, nil
}
