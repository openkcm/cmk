package mock

import (
	"reflect"
	"sync"

	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
)

type ModelInfo struct {
	Certificates          map[uuid.UUID]model.Certificate
	Groups                map[uuid.UUID]model.Group
	Keys                  map[uuid.UUID]model.Key
	KeyConfiguration      map[uuid.UUID]model.KeyConfiguration
	KeyConfigurationTags  map[uuid.UUID]model.KeyConfigurationTag
	KeystoreConfiguration map[uuid.UUID]model.KeystoreConfiguration
	KeyVersions           map[string]model.KeyVersion
	Labels                map[uuid.UUID]model.KeyLabel
	Systems               map[uuid.UUID]model.System
	Tenants               map[string]model.Tenant
	TenantConfigs         map[string]model.TenantConfig
	Workflows             map[uuid.UUID]model.Workflow
}

// InMemoryDB represents the in memory database
type InMemoryDB struct {
	Data ModelInfo
	mu   sync.RWMutex
}

// NewInMemoryDB creates and returns a nwe instance of InMemoryDB
func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		Data: ModelInfo{
			Certificates:          map[uuid.UUID]model.Certificate{},
			Groups:                map[uuid.UUID]model.Group{},
			Keys:                  map[uuid.UUID]model.Key{},
			KeyConfiguration:      map[uuid.UUID]model.KeyConfiguration{},
			KeyConfigurationTags:  map[uuid.UUID]model.KeyConfigurationTag{},
			KeystoreConfiguration: map[uuid.UUID]model.KeystoreConfiguration{},
			KeyVersions:           map[string]model.KeyVersion{},
			Labels:                map[uuid.UUID]model.KeyLabel{},
			Systems:               map[uuid.UUID]model.System{},
			Tenants:               map[string]model.Tenant{},
			TenantConfigs:         map[string]model.TenantConfig{},
			Workflows:             map[uuid.UUID]model.Workflow{},
		},
	}
}

// Create adds resource to the InMemoryDatabase
//
//nolint:cyclop,funlen
func (db *InMemoryDB) Create(resource repo.Resource) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if resource == nil {
		return ErrResourceIsNil
	}

	modelType := getType(resource)
	switch modelType {
	case Certificate:
		cert, err := GetModelFromInterface[model.Certificate](resource)
		if err != nil {
			return errs.Wrap(ErrCertificateNotFound, err)
		}

		db.Data.Certificates[cert.ID] = *cert
	case Group:
		group, err := GetModelFromInterface[model.Group](resource)
		if err != nil {
			return errs.Wrap(ErrGroupNotFound, err)
		}

		db.Data.Groups[group.ID] = *group
	case Key:
		key, err := GetModelFromInterface[model.Key](resource)
		if err != nil {
			return errs.Wrap(ErrKeyNotFound, err)
		}

		db.Data.Keys[key.ID] = *key
	case KeyConfiguration:
		keyConfiguration, err := GetModelFromInterface[model.KeyConfiguration](resource)
		if err != nil {
			return errs.Wrap(ErrKeyConfigurationNotFound, err)
		}

		db.Data.KeyConfiguration[keyConfiguration.ID] = *keyConfiguration
	case KeyConfigurationTag:
		tag, err := GetModelFromInterface[model.KeyConfigurationTag](resource)
		if err != nil {
			return errs.Wrap(ErrTagNotFound, err)
		}

		db.Data.KeyConfigurationTags[tag.ID] = *tag
	case KeystoreConfiguration:
		keyStoreConfig, err := GetModelFromInterface[model.KeystoreConfiguration](resource)
		if err != nil {
			return errs.Wrap(ErrKeystoreConfigurationNotFound, err)
		}

		db.Data.KeystoreConfiguration[keyStoreConfig.ID] = *keyStoreConfig
	case KeyVersion:
		keyVersion, err := GetModelFromInterface[model.KeyVersion](resource)
		if err != nil {
			return errs.Wrap(ErrKeyVersionNotFound, err)
		}

		db.Data.KeyVersions[keyVersion.ExternalID] = *keyVersion
	case KeyLabel:
		label, err := GetModelFromInterface[model.KeyLabel](resource)
		if err != nil {
			return errs.Wrap(ErrLabelNotFound, err)
		}

		db.Data.Labels[label.ID] = *label
	case System:
		system, err := GetModelFromInterface[model.System](resource)
		if err != nil {
			return errs.Wrap(ErrSystemNotFound, err)
		}

		db.Data.Systems[system.ID] = *system
	case Tenant:
		tenant, err := GetModelFromInterface[model.Tenant](resource)
		if err != nil {
			return errs.Wrap(ErrTenantNotFound, err)
		}

		db.Data.Tenants[tenant.DomainURL] = *tenant
	case TenantConfig:
		tenantConfig, err := GetModelFromInterface[model.TenantConfig](resource)
		if err != nil {
			return errs.Wrap(ErrTenantConfigurationNotFound, err)
		}

		db.Data.TenantConfigs[tenantConfig.Key] = *tenantConfig
	case Workflow:
		workflow, err := GetModelFromInterface[model.Workflow](resource)
		if err != nil {
			return errs.Wrap(ErrWorkflowNotFound, err)
		}

		db.Data.Workflows[workflow.ID] = *workflow
	default:
		return ErrResourceNotFound
	}

	return nil
}

// Get returns resource from InMemoryDatabase
//
//nolint:cyclop,funlen
func (db *InMemoryDB) Get(resource repo.Resource) (repo.Resource, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	modelType := getType(resource)
	switch modelType {
	case Certificate:
		result, err := db.getCertificate(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case Group:
		result, err := db.getGroup(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case Key:
		result, err := db.getKey(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case KeyConfiguration:
		result, err := db.getKeyConfiguration(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case KeyConfigurationTag:
		result, err := db.getKeyConfigurationTag(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case KeystoreConfiguration:
		result, err := db.getKeystoreConfiguration(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case KeyVersion:
		result, err := db.getKeyVersion(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case KeyLabel:
		result, err := db.getKeyLabel(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case System:
		result, err := db.getSystem(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case Tenant:
		result, err := db.getTenant(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case TenantConfig:
		result, err := db.getTenantConfig(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	case Workflow:
		result, err := db.getWorkflow(resource)
		if err != nil {
			return nil, err
		}

		return result, nil
	default:
		return nil, ErrResourceNotFound
	}
}

//nolint:cyclop,funlen
func (db *InMemoryDB) GetAll(resource any) ([]repo.Resource, int) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	modelType := getType(resource)
	switch modelType {
	case Certificate:
		certs := make([]model.Certificate, 0, len(db.Data.Certificates))
		for _, cert := range db.Data.Certificates {
			certs = append(certs, cert)
		}

		return ConvertSliceToInterface[model.Certificate](certs), len(certs)
	case Group:
		groups := make([]model.Group, 0, len(db.Data.Groups))
		for _, group := range db.Data.Groups {
			groups = append(groups, group)
		}

		return ConvertSliceToInterface[model.Group](groups), len(groups)
	case Key:
		keys := make([]model.Key, 0, len(db.Data.Keys))
		for _, key := range db.Data.Keys {
			keys = append(keys, key)
		}

		return ConvertSliceToInterface[model.Key](keys), len(keys)
	case KeyConfiguration:
		keys := make([]model.KeyConfiguration, 0, len(db.Data.KeyConfiguration))
		for _, key := range db.Data.KeyConfiguration {
			keys = append(keys, key)
		}

		return ConvertSliceToInterface[model.KeyConfiguration](keys), len(keys)
	case KeyConfigurationTag:
		tags := make([]model.KeyConfigurationTag, 0, len(db.Data.KeyConfigurationTags))
		for _, tag := range db.Data.KeyConfigurationTags {
			tags = append(tags, tag)
		}

		return ConvertSliceToInterface[model.KeyConfigurationTag](tags), len(tags)
	case KeystoreConfiguration:
		keys := make([]model.KeystoreConfiguration, 0, len(db.Data.KeystoreConfiguration))
		for _, key := range db.Data.KeystoreConfiguration {
			keys = append(keys, key)
		}

		return ConvertSliceToInterface[model.KeystoreConfiguration](keys), len(keys)
	case KeyVersion:
		keys := make([]model.KeyVersion, 0, len(db.Data.KeyVersions))
		for _, key := range db.Data.KeyVersions {
			keys = append(keys, key)
		}

		return ConvertSliceToInterface[model.KeyVersion](keys), len(keys)
	case KeyLabel:
		keyLabels := make([]model.KeyLabel, 0, len(db.Data.Labels))
		for _, keyLabel := range db.Data.Labels {
			keyLabels = append(keyLabels, keyLabel)
		}

		return ConvertSliceToInterface[model.KeyLabel](keyLabels), len(keyLabels)
	case System:
		systems := make([]model.System, 0, len(db.Data.Systems))
		for _, system := range db.Data.Systems {
			systems = append(systems, system)
		}

		return ConvertSliceToInterface[model.System](systems), len(systems)
	case Tenant:
		tenants := make([]model.Tenant, 0, len(db.Data.Tenants))
		for _, tenant := range db.Data.Tenants {
			tenants = append(tenants, tenant)
		}

		return ConvertSliceToInterface[model.Tenant](tenants), len(tenants)
	case TenantConfig:
		tenantConfigs := make([]model.TenantConfig, 0, len(db.Data.TenantConfigs))
		for _, tenantConfig := range db.Data.TenantConfigs {
			tenantConfigs = append(tenantConfigs, tenantConfig)
		}

		return ConvertSliceToInterface[model.TenantConfig](tenantConfigs), len(tenantConfigs)
	case Workflow:
		workflows := make([]model.Workflow, 0, len(db.Data.Workflows))
		for _, workflow := range db.Data.Workflows {
			workflows = append(workflows, workflow)
		}

		return ConvertSliceToInterface[model.Workflow](workflows), len(workflows)
	default:
		return nil, 0
	}
}

//nolint:cyclop,funlen
func (db *InMemoryDB) Delete(resource repo.Resource) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	modelType := getType(resource)
	switch modelType {
	case Certificate:
		done, err2 := db.deleteCertificate(resource)
		if done {
			return err2
		}
	case Group:
		done, err2 := db.deleteGroup(resource)
		if done {
			return err2
		}
	case Key:
		done, err2 := db.deleteKey(resource)
		if done {
			return err2
		}
	case KeyConfiguration:
		done, err2 := db.deleteKeyConfiguration(resource)
		if done {
			return err2
		}
	case KeyConfigurationTag:
		done, err2 := db.deleteKeyConfigurationTag(resource)
		if done {
			return err2
		}
	case KeystoreConfiguration:
		done, err2 := db.deleteKeystoreConfiguration(resource)
		if done {
			return err2
		}
	case KeyVersion:
		done, err2 := db.deleteKeyVersion(resource)
		if done {
			return err2
		}
	case KeyLabel:
		done, err2 := db.deleteKeyLabel(resource)
		if done {
			return err2
		}
	case System:
		done, err2 := db.deleteSystem(resource)
		if done {
			return err2
		}
	case TenantConfig:
		done, err2 := db.deleteTenantConfig(resource)
		if done {
			return err2
		}
	case Workflow:
		done, err2 := db.deleteWorkflow(resource)
		if done {
			return err2
		}
	default:
		return ErrResourceNotFound
	}

	return nil
}

//nolint:cyclop,funlen
func (db *InMemoryDB) Update(resource repo.Resource) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	modelType := getType(resource)
	switch modelType {
	case Certificate:
		err := db.updateCertificate(resource)
		if err != nil {
			return err
		}

		return nil
	case Group:
		err := db.updateGroup(resource)
		if err != nil {
			return err
		}

		return nil
	case Key:
		err := db.updateKey(resource)
		if err != nil {
			return err
		}

		return nil
	case KeyConfiguration:
		err := db.updateKeyConfiguration(resource)
		if err != nil {
			return err
		}

		return nil
	case KeyConfigurationTag:
		err := db.updateKeyConfigurationTag(resource)
		if err != nil {
			return err
		}

		return nil
	case KeystoreConfiguration:
		err := db.updateKeystoreConfiguration(resource)
		if err != nil {
			return err
		}

		return nil
	case KeyVersion:
		err := db.updateKeyVersion(resource)
		if err != nil {
			return err
		}

		return nil
	case KeyLabel:
		err := db.updateKeyLabel(resource)
		if err != nil {
			return err
		}

		return nil
	case System:
		err := db.updateSystem(resource)
		if err != nil {
			return err
		}

		return nil
	case TenantConfig:
		err := db.updateTenantConfig(resource)
		if err != nil {
			return err
		}

		return nil
	case Workflow:
		err := db.updateWorkflow(resource)
		if err != nil {
			return err
		}

		return nil
	}

	return ErrResourceNotFound
}

func (db *InMemoryDB) getWorkflow(resource repo.Resource) (repo.Resource, error) {
	resourceWorkflow, err := GetModelFromInterface[model.Workflow](resource)
	if err != nil {
		return nil, errs.Wrap(ErrWorkflowNotFound, err)
	}

	for _, workflow := range db.Data.Workflows {
		if workflow.ID == resourceWorkflow.ID {
			return workflow, nil
		}
	}

	return nil, errs.Wrap(ErrWorkflowNotFound, err)
}

func (db *InMemoryDB) getTenantConfig(resource repo.Resource) (repo.Resource, error) {
	resourceTenantConfig, err := GetModelFromInterface[model.TenantConfig](resource)
	if err != nil {
		return nil, errs.Wrap(ErrTenantConfigurationNotFound, err)
	}

	for _, tenantConfig := range db.Data.TenantConfigs {
		if tenantConfig.Key == resourceTenantConfig.Key {
			return tenantConfig, nil
		}
	}

	return nil, errs.Wrap(ErrTenantConfigurationNotFound, err)
}

func (db *InMemoryDB) getTenant(resource repo.Resource) (repo.Resource, error) {
	resourceTenant, err := GetModelFromInterface[model.Tenant](resource)
	if err != nil {
		return nil, errs.Wrap(ErrTenantNotFound, err)
	}

	for _, tenant := range db.Data.Tenants {
		if tenant.DomainURL == resourceTenant.DomainURL {
			return tenant, nil
		}
	}

	return nil, errs.Wrap(ErrTenantNotFound, err)
}

func (db *InMemoryDB) getKeyConfigurationTag(resource repo.Resource) (repo.Resource, error) {
	resourceTag, err := GetModelFromInterface[model.KeyConfigurationTag](resource)
	if err != nil {
		return nil, errs.Wrap(ErrTagNotFound, err)
	}

	for _, tag := range db.Data.KeyConfigurationTags {
		if tag.ID == resourceTag.ID {
			return tag, nil
		}
	}

	return nil, errs.Wrap(ErrTagNotFound, err)
}

func (db *InMemoryDB) getSystem(resource repo.Resource) (repo.Resource, error) {
	resourceSystem, err := GetModelFromInterface[model.System](resource)
	if err != nil {
		return nil, errs.Wrap(ErrSystemNotFound, err)
	}

	for _, system := range db.Data.Systems {
		if system.ID == resourceSystem.ID {
			return system, nil
		}
	}

	return nil, errs.Wrap(ErrSystemNotFound, err)
}

func (db *InMemoryDB) getKeyLabel(resource repo.Resource) (repo.Resource, error) {
	resourceLabel, err := GetModelFromInterface[model.KeyLabel](resource)
	if err != nil {
		return nil, errs.Wrap(ErrLabelNotFound, err)
	}

	for _, label := range db.Data.Labels {
		if label.ID == resourceLabel.ID {
			return label, nil
		}
	}

	return nil, errs.Wrap(ErrLabelNotFound, err)
}

func (db *InMemoryDB) getKeyVersion(resource repo.Resource) (repo.Resource, error) {
	resourceKeyVersion, err := GetModelFromInterface[model.KeyVersion](resource)
	if err != nil {
		return nil, errs.Wrap(ErrKeyVersionNotFound, err)
	}

	for _, keyVersion := range db.Data.KeyVersions {
		if keyVersion.ExternalID == resourceKeyVersion.ExternalID {
			return keyVersion, nil
		}
	}

	return nil, errs.Wrap(ErrKeyVersionNotFound, err)
}

func (db *InMemoryDB) getKeystoreConfiguration(resource repo.Resource) (repo.Resource, error) {
	resourceKey, err := GetModelFromInterface[model.KeystoreConfiguration](resource)
	if err != nil {
		return nil, errs.Wrap(ErrKeystoreConfigurationNotFound, err)
	}

	for _, key := range db.Data.KeystoreConfiguration {
		if key.ID == resourceKey.ID {
			return key, nil
		}
	}

	return nil, errs.Wrap(ErrKeystoreConfigurationNotFound, err)
}

func (db *InMemoryDB) getKeyConfiguration(resource repo.Resource) (repo.Resource, error) {
	resourceKey, err := GetModelFromInterface[model.KeyConfiguration](resource)
	if err != nil {
		return nil, errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	for _, key := range db.Data.KeyConfiguration {
		if key.ID == resourceKey.ID {
			return key, nil
		}
	}

	return nil, errs.Wrap(ErrKeyConfigurationNotFound, err)
}

func (db *InMemoryDB) getKey(resource repo.Resource) (repo.Resource, error) {
	resourceKey, err := GetModelFromInterface[model.Key](resource)
	if err != nil {
		return nil, errs.Wrap(ErrKeyNotFound, err)
	}

	for _, key := range db.Data.Keys {
		if key.ID == resourceKey.ID {
			return key, nil
		}
	}

	return nil, errs.Wrap(ErrKeyNotFound, err)
}

func (db *InMemoryDB) getGroup(resource repo.Resource) (repo.Resource, error) {
	resourceGroup, err := GetModelFromInterface[model.Group](resource)
	if err != nil {
		return nil, errs.Wrap(ErrGroupNotFound, err)
	}

	for _, group := range db.Data.Groups {
		if group.ID == resourceGroup.ID {
			return group, nil
		}
	}

	return nil, errs.Wrap(ErrGroupNotFound, err)
}

func (db *InMemoryDB) getCertificate(resource repo.Resource) (repo.Resource, error) {
	resourceCertificate, err := GetModelFromInterface[model.Certificate](resource)
	if err != nil {
		return nil, errs.Wrap(ErrCertificateNotFound, err)
	}

	for _, cert := range db.Data.Certificates {
		if cert.ID == resourceCertificate.ID {
			return cert, nil
		}
	}

	return nil, errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) deleteWorkflow(resource repo.Resource) (bool, error) {
	resourceWorkflow, err := GetModelFromInterface[model.Workflow](resource)
	if err != nil {
		return true, errs.Wrap(ErrWorkflowNotFound, err)
	}

	for _, workflow := range db.Data.Workflows {
		if workflow.ID == resourceWorkflow.ID {
			delete(db.Data.Workflows, workflow.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteTenantConfig(resource repo.Resource) (bool, error) {
	resourceTenantConfig, err := GetModelFromInterface[model.TenantConfig](resource)
	if err != nil {
		return true, errs.Wrap(ErrTenantConfigurationNotFound, err)
	}

	for _, tenantConfig := range db.Data.TenantConfigs {
		if tenantConfig.Key == resourceTenantConfig.Key {
			delete(db.Data.TenantConfigs, tenantConfig.Key)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKeyConfigurationTag(resource repo.Resource) (bool, error) {
	resourceTag, err := GetModelFromInterface[model.KeyConfigurationTag](resource)
	if err != nil {
		return true, errs.Wrap(ErrTagNotFound, err)
	}

	for _, tag := range db.Data.KeyConfigurationTags {
		if tag.ID == resourceTag.ID {
			delete(db.Data.KeyConfigurationTags, tag.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteSystem(resource repo.Resource) (bool, error) {
	resourceSystem, err := GetModelFromInterface[model.System](resource)
	if err != nil {
		return true, errs.Wrap(ErrSystemNotFound, err)
	}

	for _, system := range db.Data.Systems {
		if system.ID == resourceSystem.ID {
			delete(db.Data.Systems, system.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKeyLabel(resource repo.Resource) (bool, error) {
	resourceLabel, err := GetModelFromInterface[model.KeyLabel](resource)
	if err != nil {
		return true, errs.Wrap(ErrLabelNotFound, err)
	}

	for _, label := range db.Data.Labels {
		if label.ID == resourceLabel.ID {
			delete(db.Data.Labels, label.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKeyVersion(resource repo.Resource) (bool, error) {
	resourceKey, err := GetModelFromInterface[model.KeyVersion](resource)
	if err != nil {
		return true, errs.Wrap(ErrKeyVersionNotFound, err)
	}

	for _, keyVersion := range db.Data.KeyVersions {
		if keyVersion.ExternalID == resourceKey.ExternalID {
			delete(db.Data.KeyVersions, keyVersion.ExternalID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKeystoreConfiguration(resource repo.Resource) (bool, error) {
	resourceKey, err := GetModelFromInterface[model.KeystoreConfiguration](resource)
	if err != nil {
		return true, errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	for _, keystoreConfiguration := range db.Data.KeystoreConfiguration {
		if keystoreConfiguration.ID == resourceKey.ID {
			delete(db.Data.KeystoreConfiguration, keystoreConfiguration.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKeyConfiguration(resource repo.Resource) (bool, error) {
	resourceKey, err := GetModelFromInterface[model.KeyConfiguration](resource)
	if err != nil {
		return true, errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	for _, keyConfiguration := range db.Data.KeyConfiguration {
		if keyConfiguration.ID == resourceKey.ID {
			delete(db.Data.KeyConfiguration, keyConfiguration.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteKey(resource repo.Resource) (bool, error) {
	resourceKey, err := GetModelFromInterface[model.Key](resource)
	if err != nil {
		return true, errs.Wrap(ErrKeyNotFound, err)
	}

	for _, key := range db.Data.Keys {
		if key.ID == resourceKey.ID {
			delete(db.Data.Keys, key.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteGroup(resource repo.Resource) (bool, error) {
	groupResource, err := GetModelFromInterface[model.Group](resource)
	if err != nil {
		return true, errs.Wrap(ErrGroupNotFound, err)
	}

	for _, group := range db.Data.Groups {
		if group.ID == groupResource.ID {
			delete(db.Data.Groups, group.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) deleteCertificate(resource repo.Resource) (bool, error) {
	resourceCertificate, err := GetModelFromInterface[model.Certificate](resource)
	if err != nil {
		return true, errs.Wrap(ErrCertificateNotFound, err)
	}

	for _, cert := range db.Data.Certificates {
		if cert.ID == resourceCertificate.ID {
			delete(db.Data.Certificates, cert.ID)
			return true, nil
		}
	}

	return false, nil
}

func (db *InMemoryDB) updateWorkflow(resource repo.Resource) error {
	resourceWorkflow, err := GetModelFromInterface[model.Workflow](resource)
	if err != nil {
		return errs.Wrap(ErrWorkflowNotFound, err)
	}

	for _, workflow := range db.Data.Workflows {
		if workflow.ID == resourceWorkflow.ID {
			db.Data.Workflows[workflow.ID] = *resourceWorkflow
			return nil
		}
	}

	return errs.Wrap(ErrTagNotFound, err)
}

func (db *InMemoryDB) updateTenantConfig(resource repo.Resource) error {
	resourceTenantConfig, err := GetModelFromInterface[model.TenantConfig](resource)
	if err != nil {
		return errs.Wrap(ErrTenantConfigurationNotFound, err)
	}

	for _, tenantConfig := range db.Data.TenantConfigs {
		if tenantConfig.Key == resourceTenantConfig.Key {
			db.Data.TenantConfigs[tenantConfig.Key] = *resourceTenantConfig
			return nil
		}
	}

	return errs.Wrap(ErrTagNotFound, err)
}

func (db *InMemoryDB) updateKeyConfigurationTag(resource repo.Resource) error {
	resourceTag, err := GetModelFromInterface[model.KeyConfigurationTag](resource)
	if err != nil {
		return errs.Wrap(ErrTagNotFound, err)
	}

	for _, tag := range db.Data.KeyConfigurationTags {
		if tag.ID == resourceTag.ID {
			db.Data.KeyConfigurationTags[tag.ID] = *resourceTag
			return nil
		}
	}

	return errs.Wrap(ErrTagNotFound, err)
}

func (db *InMemoryDB) updateSystem(resource repo.Resource) error {
	resourceSystem, err := GetModelFromInterface[model.System](resource)
	if err != nil {
		return errs.Wrap(ErrSystemNotFound, err)
	}

	for _, system := range db.Data.Systems {
		if system.ID == resourceSystem.ID {
			db.Data.Systems[system.ID] = *resourceSystem
			return nil
		}
	}

	return errs.Wrap(ErrSystemNotFound, err)
}

func (db *InMemoryDB) updateKeyLabel(resource repo.Resource) error {
	resourceKeyLabel, err := GetModelFromInterface[model.KeyLabel](resource)
	if err != nil {
		return errs.Wrap(ErrLabelNotFound, err)
	}

	for _, keyLabel := range db.Data.Labels {
		if keyLabel.ID == resourceKeyLabel.ID {
			db.Data.Labels[keyLabel.ID] = *resourceKeyLabel
			return nil
		}
	}

	return errs.Wrap(ErrTagNotFound, err)
}

func (db *InMemoryDB) updateKeyVersion(resource repo.Resource) error {
	resourceKeyVersion, err := GetModelFromInterface[model.KeyVersion](resource)
	if err != nil {
		return errs.Wrap(ErrKeyVersionNotFound, err)
	}

	for _, keyVersion := range db.Data.KeyVersions {
		if keyVersion.ExternalID == resourceKeyVersion.ExternalID {
			db.Data.KeyVersions[keyVersion.ExternalID] = *resourceKeyVersion
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) updateKeystoreConfiguration(resource repo.Resource) error {
	resourceKeyConfiguration, err := GetModelFromInterface[model.KeystoreConfiguration](resource)
	if err != nil {
		return errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	for _, keystoreConfiguration := range db.Data.KeystoreConfiguration {
		if keystoreConfiguration.ID == resourceKeyConfiguration.ID {
			db.Data.KeystoreConfiguration[keystoreConfiguration.ID] = *resourceKeyConfiguration
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) updateKeyConfiguration(resource repo.Resource) error {
	resourceKeyConfiguration, err := GetModelFromInterface[model.KeyConfiguration](resource)
	if err != nil {
		return errs.Wrap(ErrKeyConfigurationNotFound, err)
	}

	for _, keyConfiguration := range db.Data.KeyConfiguration {
		if keyConfiguration.ID == resourceKeyConfiguration.ID {
			db.Data.KeyConfiguration[keyConfiguration.ID] = *resourceKeyConfiguration
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) updateKey(resource repo.Resource) error {
	resourceKey, err := GetModelFromInterface[model.Key](resource)
	if err != nil {
		return errs.Wrap(ErrKeyNotFound, err)
	}

	for _, key := range db.Data.Keys {
		if key.ID == resourceKey.ID {
			db.Data.Keys[key.ID] = *resourceKey
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) updateGroup(resource repo.Resource) error {
	resourceGroup, err := GetModelFromInterface[model.Group](resource)
	if err != nil {
		return errs.Wrap(ErrGroupNotFound, err)
	}

	for _, group := range db.Data.Groups {
		if group.ID == resourceGroup.ID {
			db.Data.Groups[group.ID] = *resourceGroup
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func (db *InMemoryDB) updateCertificate(resource repo.Resource) error {
	resourceCertificate, err := GetModelFromInterface[model.Certificate](resource)
	if err != nil {
		return errs.Wrap(ErrCertificateNotFound, err)
	}

	for _, cert := range db.Data.Certificates {
		if cert.ID == resourceCertificate.ID {
			db.Data.Certificates[cert.ID] = *resourceCertificate
			return nil
		}
	}

	return errs.Wrap(ErrCertificateNotFound, err)
}

func getType(val any) string {
	t := reflect.TypeOf(val)

	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}

	return t.Name()
}

func ConvertSliceToModel[T any](input []repo.Resource) ([]T, error) {
	result := make([]T, 0, len(input))

	for _, item := range input {
		val, ok := item.(T)
		if !ok {
			return nil, errs.Wrapf(ErrFormatResourceIsNot, getType(item))
		}

		result = append(result, val)
	}

	return result, nil
}

func ConvertSliceToInterface[T repo.Resource](input []T) []repo.Resource {
	result := make([]repo.Resource, 0, len(input))
	for _, item := range input {
		result = append(result, item)
	}

	return result
}

func GetModelFromInterface[T repo.Resource](resource any) (*T, error) {
	resourceKey, ok := resource.(T)
	if !ok {
		ptrResource, ok := resource.(*T)
		if !ok {
			return nil, errs.Wrapf(ErrFormatResourceIsNot, getType(resource))
		}

		resourceKey = *ptrResource
	}

	return &resourceKey, nil
}
