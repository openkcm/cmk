package async_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	asyncUtils "github.com/openkcm/cmk/utils/async"
	ctxUtils "github.com/openkcm/cmk/utils/context"
)

func TestNewTaskPayload(t *testing.T) {
	ctx := ctxUtils.CreateTenantContext(context.Background(), "tenant-123")

	data := []byte("payload")
	payload := asyncUtils.NewTaskPayload(ctx, data)

	assert.Equal(t, "tenant-123", payload.TenantID)
	assert.Equal(t, data, payload.Data)
}

func TestParseTaskPayload_Success(t *testing.T) {
	payload := asyncUtils.TaskPayload{TenantID: "t1", Data: []byte("abc")}
	bytes, err := json.Marshal(payload)
	assert.NoError(t, err)

	newPayload, err := asyncUtils.ParseTaskPayload(bytes)

	assert.NoError(t, err)
	assert.Equal(t, newPayload.TenantID, payload.TenantID)
	assert.Equal(t, newPayload.Data, payload.Data)
}

func TestParseTaskPayload_Fail(t *testing.T) {
	_, err := asyncUtils.ParseTaskPayload([]byte("{invalid json"))
	assert.Error(t, err)
}

func TestTaskPayload_ToBytes(t *testing.T) {
	payload := asyncUtils.TaskPayload{TenantID: "t2", Data: []byte("xyz")}
	bytes, err := payload.ToBytes()
	assert.NoError(t, err)

	var newPayload asyncUtils.TaskPayload

	err = json.Unmarshal(bytes, &newPayload)
	assert.NoError(t, err)

	assert.Equal(t, newPayload.TenantID, payload.TenantID)
	assert.Equal(t, newPayload.Data, payload.Data)
}

func TestTaskPayload_InjectContext(t *testing.T) {
	payload := asyncUtils.TaskPayload{TenantID: "tenant-ctx"}
	ctx := context.Background()
	ctx = payload.InjectContext(ctx)
	assert.NotNil(t, ctx)

	tenantID, err := ctxUtils.ExtractTenantID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "tenant-ctx", tenantID)
}
