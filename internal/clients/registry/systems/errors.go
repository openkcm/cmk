package systems

import "errors"

var (
	ErrSystemsClientDoesNotExist           = errors.New("systems client does not exist")
	ErrSystemsClientFailedGettingSystems   = errors.New("systems client failed to list systems")
	ErrSystemsClientFailedMappingSystems   = errors.New("systems client failed to map systems")
	ErrSystemsClientFailedUpdatingKeyClaim = errors.New("systems client failed to update keyclaim")
	ErrSystemsServerFailedUpdatingKeyClaim = errors.New("systems server failed to update systems key claim")
	ErrSystemsClientFailedUpdatingStatus   = errors.New("systems client failed to update system status")
	ErrSystemsServerFailedUpdatingStatus   = errors.New("systems server failed to update system status")
)
