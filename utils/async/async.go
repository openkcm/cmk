package async

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	ctxUtils "github.com/openkcm/cmk/utils/context"
)

var (
	ErrParsingPayload = errors.New("could not parse task payload")
)

// TaskPayload represents the payload for an async task, including tenant context.
type TaskPayload struct {
	TenantID    string
	AuthContext map[string]string
	Data        []byte
}

func NewTaskPayload(ctx context.Context, data []byte) TaskPayload {
	tenantID, err := ctxUtils.ExtractTenantID(ctx)
	if err != nil {
		tenantID = ""
	}

	authContext, err := ctxUtils.ExtractClientDataAuthContext(ctx)
	if err != nil {
		authContext = nil
	}

	return TaskPayload{
		TenantID:    tenantID,
		AuthContext: authContext,
		Data:        data,
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

	if p.AuthContext != nil {
		ctx = context.WithValue(ctx, constants.ClientData, &auth.ClientData{AuthContext: p.AuthContext})
	}

	return ctx
}

func (p *TaskPayload) ToBytes() ([]byte, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, errs.Wrap(ErrParsingPayload, err)
	}

	return data, nil
}
