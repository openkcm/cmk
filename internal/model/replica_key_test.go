package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/model"
)

func TestReplicaKey(t *testing.T) {
	replicaKey := model.ReplicaKey{}
	t.Run("Authz resource is key", func(t *testing.T) {
		assert.Equal(t, authz.RepoResourceTypeKey, replicaKey.TableResourceType())
	})
	t.Run("Key and KeyReplica has same Authz", func(t *testing.T) {
		assert.Equal(t, authz.RepoResourceTypeKey, replicaKey.TableResourceType())
	})
}
