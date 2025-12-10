package keyconfiguration

import (
	"errors"
	"reflect"

	"github.com/google/uuid"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/transform"
	"github.tools.sap/kms/cmk/internal/api/transform/group"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/sanitise"
	"github.tools.sap/kms/cmk/utils/validator"
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
	err := sanitise.Stringlikes(&k)
	if err != nil {
		return nil, err
	}

	apiConfig := &cmkapi.KeyConfiguration{
		Id:           &k.ID,
		Name:         k.Name,
		AdminGroupID: k.AdminGroupID,
		PrimaryKeyID: k.PrimaryKeyID,
	}

	if !reflect.ValueOf(k.AdminGroup).IsZero() {
		adminGroup, err := group.ToAPI(k.AdminGroup)
		if err != nil {
			return nil, err
		}

		apiConfig.AdminGroup = adminGroup
	}

	if k.Description != "" {
		apiConfig.Description = &k.Description
	}

	apiConfig.Metadata = &cmkapi.KeyConfigurationMetadata{
		CreatedAt:    &k.CreatedAt,
		UpdatedAt:    &k.UpdatedAt,
		CreatorID:    &k.CreatorID,
		CreatorName:  &k.CreatorName,
		TotalKeys:    &k.TotalKeys,
		TotalSystems: &k.TotalSystems,
	}

	systemConnect := k.PrimaryKeyID != nil
	apiConfig.CanConnectSystems = &systemConnect

	return apiConfig, nil
}
