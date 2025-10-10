package config

import (
	"errors"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrConfigurationValuesError        = errors.New("configuration value error")
	ErrCertificateValidityOutsideRange = errors.New("certificate validity must not be outside 7 to 30 days")
	ErrNonDefinedTaskType              = errors.New("task type is unknown")
	ErrRepeatedTaskType                = errors.New("task type is specified more than once")

	ErrAMQPEmptyURL      = errors.New("AMQP URL must be specified")
	ErrAMQPEmptyTarget   = errors.New("AMQP target must be specified")
	ErrAMQPEmptySource   = errors.New("AMQP source must be specified")
	ErrTargetEmptyRegion = errors.New("target region must be specified")
)

// Config holds all application configuration parameters
type Config struct {
	commoncfg.BaseConfig `mapstructure:",squash"`

	Database         Database                     `yaml:"database"`
	DatabaseReplicas []Database                   `yaml:"databaseReplicas"`
	Scheduler        Scheduler                    `yaml:"scheduler"`
	HTTP             HTTPServer                   `yaml:"http"`
	Plugins          []plugincatalog.PluginConfig `yaml:"plugins"`
	Services         Services                     `yaml:"services"`
	Workflows        Workflows                    `yaml:"workflows"`
	Certificates     Certificates                 `yaml:"certificates"`
	Provisioning     Provisioning                 `yaml:"provisioning"`
	System           System
	ClientData       ClientData `yaml:"clientData"`

	CryptoLayer CryptoLayer `yaml:"cryptoLayer"`

	TenantManager  TenantManager  `yaml:"tenantManager"`
	EventProcessor EventProcessor `yaml:"eventProcessor"`

	KeystorePool KeystorePool `yaml:"keystorePool"`
}

func (c *Config) Validate() error {
	err := c.Scheduler.Validate()
	if err != nil {
		return errs.Wrap(ErrConfigurationValuesError, err)
	}

	err = c.Certificates.Validate()
	if err != nil {
		return errs.Wrap(ErrConfigurationValuesError, err)
	}

	return nil
}

type CryptoLayer struct {
	CertX509Trusts commoncfg.SourceRef `yaml:"certX509Trusts"`
}

// Certificates holds certificates config
type Certificates struct {
	RootCertURL           string
	ValidityDays          int
	RotationThresholdDays int
}

const (
	MinCertificateValidityDays = 7
	MaxCertificateValidityDays = 30
)

func (c *Certificates) Validate() error {
	if c.ValidityDays < MinCertificateValidityDays ||
		c.ValidityDays > MaxCertificateValidityDays {
		return ErrCertificateValidityOutsideRange
	}

	return nil
}

// Scheduler holds a scheduler config
type Scheduler struct {
	TaskQueue Redis
	Tasks     []Task
}

func (s *Scheduler) Validate() error {
	checkedTasks := make(map[string]struct{}, len(s.Tasks))
	for _, task := range s.Tasks {
		_, found := DefinedTasks[task.TaskType]
		if !found {
			return ErrNonDefinedTaskType
		}

		_, found = checkedTasks[task.TaskType]
		if found {
			return ErrRepeatedTaskType
		}

		checkedTasks[task.TaskType] = struct{}{}
	}

	return nil
}

// Task holds a task config
type Task struct {
	Cronspec string
	TaskType string
	Retries  int
}

// Redis holds Redis client config
type Redis struct {
	Host      commoncfg.SourceRef `yaml:"host"`
	Port      string              `yaml:"port"`
	ACL       RedisACL            `yaml:"acl"`
	SecretRef commoncfg.SecretRef
}

type RedisACL struct {
	Enabled  bool                `yaml:"enabled"`
	Password commoncfg.SourceRef `yaml:"password"`
	Username commoncfg.SourceRef `yaml:"username"`
}

// Database holds database config
type Database struct {
	Name   string              `yaml:"name"`
	Port   string              `yaml:"port"`
	Host   commoncfg.SourceRef `yaml:"host"`
	User   commoncfg.SourceRef `yaml:"user"`
	Secret commoncfg.SourceRef `yaml:"secret"`
}

type System struct {
	Identifier         SystemProperty            `yaml:"identifier" mapstructure:"identifier"`
	Region             SystemProperty            `yaml:"region"  mapstructure:"region"`
	Type               SystemProperty            `yaml:"type" mapstructure:"type"`
	OptionalProperties map[string]SystemProperty `yaml:",inline" mapstructure:",remain"`
}

type SystemProperty struct {
	DisplayName string `yaml:"displayName" mapstructure:"displayName"`
	Internal    bool   `yaml:"internal" mapstructure:"internal"`
	Optional    bool   `yaml:"optional" mapstructure:"optional"`
	Default     any    `yaml:"default" mapstructure:"default"`
}

// Services holds services config
type Services struct {
	Registry *commoncfg.GRPCClient `yaml:"registry" mapstructure:"registry"`
}

// HTTPServer holds http server config
type HTTPServer struct {
	Address         string        `yaml:"address" default:":8080"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout" default:"5s"`
}

// Workflows holds workflows config
type Workflows struct {
	// Enabled determines if workflows are enabled in controllers
	Enabled bool

	// MinimumApprovals is the minimum number of approvals required for a workflow
	MinimumApprovals int
}

type TenantManager struct {
	SecretRef commoncfg.SecretRef `yaml:"secretRef"`
	AMQP      AMQP                `yaml:"amqp"`
}

// Validate checks the TenantManager configuration values
func (t *TenantManager) Validate() error {
	if t.SecretRef.Type != commoncfg.MTLSSecretType && t.SecretRef.Type != commoncfg.InsecureSecretType {
		return errs.Wrapf(ErrConfigurationValuesError, "only insecure or mtls secrets are supported for tenant manager")
	}

	err := t.AMQP.validate()
	if err != nil {
		return errs.Wrap(ErrConfigurationValuesError, err)
	}

	return nil
}

type EventProcessor struct {
	SecretRef         commoncfg.SecretRef `yaml:"secretRef"`
	Targets           []Target            `yaml:"targets"`
	MaxReconcileCount int64               `yaml:"maxReconcileCount"`
}

// Validate checks the EventProcessor configuration values
func (e *EventProcessor) Validate() error {
	if e.SecretRef.Type != commoncfg.MTLSSecretType && e.SecretRef.Type != commoncfg.InsecureSecretType {
		return errs.Wrapf(
			ErrConfigurationValuesError, "only insecure or mtls secrets are supported for event processor",
		)
	}

	for _, target := range e.Targets {
		err := target.validate()
		if err != nil {
			return errs.Wrap(ErrConfigurationValuesError, err)
		}
	}

	return nil
}

type Target struct {
	Region string `yaml:"region"`
	AMQP   AMQP   `yaml:"amqp"`
}

func (t *Target) validate() error {
	if t.Region == "" {
		return ErrTargetEmptyRegion
	}

	return t.AMQP.validate()
}

type AMQP struct {
	URL    string `yaml:"url"`
	Target string `yaml:"target"`
	Source string `yaml:"source"`
}

func (a *AMQP) validate() error {
	if a.URL == "" {
		return ErrAMQPEmptyURL
	}

	if a.Target == "" {
		return ErrAMQPEmptyTarget
	}

	if a.Source == "" {
		return ErrAMQPEmptySource
	}

	return nil
}

// Provisioning config of application
type Provisioning struct {
	InitKeystoreConfig InitKeystoreConfig
}

// ClientData holds signing keys path for client data signature verification
// and other client data fields
type ClientData struct {
	// SigningKeysPath is the path where signing keys are mounted
	SigningKeysPath string   `yaml:"signingKeysPath"`
	Subject         string   `yaml:"subject"`
	Groups          []string `yaml:"groups"`
	Email           string   `yaml:"email"`
	Region          string   `yaml:"region"`
	Type            string   `yaml:"type"`
}

type InitKeystoreConfig struct {
	Enabled  bool
	Provider string              `yaml:"provider"`
	Value    KeystoreConfigValue `yaml:"value"`
}

type KeystoreConfigValue struct {
	LocalityID           string   `yaml:"localityId" json:"localityId"` //nolint:tagliatelle
	CommonName           string   `yaml:"commonName" json:"commonName"`
	ManagementAccessData any      `yaml:"managementAccessData" json:"managementAccessData"`
	SupportedRegions     []Region `yaml:"supportedRegions" json:"supportedRegions"`
}

type Region struct {
	Name          string `json:"name"`
	TechnicalName string `json:"technicalName"`
}

type KeystorePool struct {
	Size     int           `yaml:"size" default:"5"`
	Interval time.Duration `yaml:"interval" default:"1h"`
}
