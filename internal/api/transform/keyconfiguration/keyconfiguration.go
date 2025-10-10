package keyconfiguration

import (
	"errors"
	"reflect"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/group"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/validator"
)

var ErrTransformKey = errors.New("err transform to key response")

// FromAPI converts a KeyConfiguration api model to a KeyConfiguration db model.
func FromAPI(apiConfig cmkapi.KeyConfiguration) (*model.KeyConfiguration, error) {
	if apiConfig.Name == "" {
		return nil, errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "name")
	}

	if apiConfig.AdminGroupID == uuid.Nil {
		return nil, errs.Wrapf(apierrors.ErrNameFieldMissingProperty, "adminGroupID")
	}

	err := validator.ValidateUUID(apiConfig.AdminGroupID.String())
	if err != nil {
		return nil, errs.Wrapf(transform.ErrAPIInvalidProperty, "adminGroupID must be UUID string")
	}

	dbConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         apiConfig.Name,
		AdminGroupID: apiConfig.AdminGroupID,
	}

	if apiConfig.Description != nil {
		dbConfig.Description = *apiConfig.Description
	}

	return dbConfig, nil
}

// ToAPI converts KeyConfiguration db model to a KeyConfiguration api model
func ToAPI(k model.KeyConfiguration) (*cmkapi.KeyConfiguration, error) {
	apiConfig := &cmkapi.KeyConfiguration{
		Id:           &k.ID,
		Name:         k.Name,
		AdminGroupID: k.AdminGroupID,
		PrimaryKeyID: k.PrimaryKeyID,
	}

	if !reflect.ValueOf(k.AdminGroup).IsZero() {
		apiConfig.AdminGroup = group.ToAPI(k.AdminGroup)
	}

	if k.Description != "" {
		apiConfig.Description = &k.Description
	}

	createdAt := k.CreatedAt.Format(transform.DefTimeFormat)
	updatedAt := k.UpdatedAt.Format(transform.DefTimeFormat)

	apiConfig.Metadata = &cmkapi.KeyConfigurationMetadata{
		CreatedAt:    &createdAt,
		UpdatedAt:    &updatedAt,
		CreatorID:    &k.CreatorID,
		CreatorName:  &k.CreatorName,
		TotalKeys:    &k.TotalKeys,
		TotalSystems: &k.TotalSystems,
	}

	systemConnect := k.PrimaryKeyID != nil
	apiConfig.CanConnectSystems = &systemConnect

	return apiConfig, nil
}
