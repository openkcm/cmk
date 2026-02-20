package model_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestWithLogInjectTenant(t *testing.T) {
	tenant := &model.Tenant{
		ID:     "tenant1",
		Region: "us-east-1",
	}
	ctx := context.Background()
	opt := model.WithLogInjectTenant(tenant)
	newCtx := opt(ctx)
	assert.NotNil(t, newCtx)
}

func TestWithLogInjectGroups(t *testing.T) {
	groups := []*model.Group{
		{IAMIdentifier: "group1"},
		{IAMIdentifier: "group2"},
	}
	ctx := context.Background()
	opt := model.WithLogInjectGroups(groups)
	newCtx := opt(ctx)
	assert.NotNil(t, newCtx)
}

func TestWithLogInjectKey(t *testing.T) {
	key := &model.Key{ID: uuid.New()}
	ctx := context.Background()
	opt := model.WithLogInjectKey(key)
	newCtx := opt(ctx)
	assert.NotNil(t, newCtx)
}

func TestWithLogInjectSystem(t *testing.T) {
	sys := &model.System{
		ID:         uuid.New(),
		Identifier: "sys1",
		Type:       "type1",
		Region:     "region1",
	}
	ctx := context.Background()
	opt := model.WithLogInjectSystem(sys)
	newCtx := opt(ctx)
	assert.NotNil(t, newCtx)
}
