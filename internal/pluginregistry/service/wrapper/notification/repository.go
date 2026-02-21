package notification

import (
	"errors"

	"github.com/openkcm/cmk/internal/pluginregistry/service/api/notification"
)

var ErrNotConfigured = errors.New("notification plugin not configured")

type Repository struct {
	Instance notification.Notification
}

func (repo *Repository) Notification() (notification.Notification, error) {
	if repo.Instance == nil {
		return nil, ErrNotConfigured
	}
	return repo.Instance, nil
}

func (repo *Repository) SetNotification(instance notification.Notification) {
	repo.Instance = instance
}

func (repo *Repository) Clear() {
	repo.Instance = nil
}
