package cmkplugincatalog

import (
	"reflect"
	"testing"

	"github.com/openkcm/plugin-sdk/api/service/certificateissuer"
	"github.com/openkcm/plugin-sdk/api/service/identitymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keymanagement"
	"github.com/openkcm/plugin-sdk/api/service/keystoremanagement"
	"github.com/openkcm/plugin-sdk/api/service/notification"
	"github.com/openkcm/plugin-sdk/api/service/systeminformation"

	serviceapi "github.com/openkcm/plugin-sdk/api/service"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
)

// mockServiceAPIRegistry acts as a fake underlying registry to control the boolean returns.
// By embedding the serviceapi.Registry interface, we avoid needing to implement
// unrelated methods (like io.Closer) just to satisfy the compiler.
type mockServiceAPIRegistry struct {
	serviceapi.Registry

	certIssuer   certificateissuer.CertificateIssuer
	certIssuerOk bool

	notification   notification.Notification
	notificationOk bool

	sysInfo   systeminformation.SystemInformation
	sysInfoOk bool

	identityMgmt   identitymanagement.IdentityManagement
	identityMgmtOk bool

	keystoreMap   map[string]keystoremanagement.KeystoreManagement
	keystoreMapOk bool

	keystoreList   []keystoremanagement.KeystoreManagement
	keystoreListOk bool

	keyMgmtMap   map[string]keymanagement.KeyManagement
	keyMgmtMapOk bool

	keyMgmtList   []keymanagement.KeyManagement
	keyMgmtListOk bool
}

func (m *mockServiceAPIRegistry) CertificateIssuer() (certificateissuer.CertificateIssuer, bool) {
	return m.certIssuer, m.certIssuerOk
}

func (m *mockServiceAPIRegistry) Notification() (notification.Notification, bool) {
	return m.notification, m.notificationOk
}

func (m *mockServiceAPIRegistry) SystemInformation() (systeminformation.SystemInformation, bool) {
	return m.sysInfo, m.sysInfoOk
}

func (m *mockServiceAPIRegistry) IdentityManagement() (identitymanagement.IdentityManagement, bool) {
	return m.identityMgmt, m.identityMgmtOk
}

func (m *mockServiceAPIRegistry) KeystoreManagements() (map[string]keystoremanagement.KeystoreManagement, bool) {
	return m.keystoreMap, m.keystoreMapOk
}

func (m *mockServiceAPIRegistry) KeystoreManagementList() ([]keystoremanagement.KeystoreManagement, bool) {
	return m.keystoreList, m.keystoreListOk
}

func (m *mockServiceAPIRegistry) KeyManagements() (map[string]keymanagement.KeyManagement, bool) {
	return m.keyMgmtMap, m.keyMgmtMapOk
}

func (m *mockServiceAPIRegistry) KeyManagementList() ([]keymanagement.KeyManagement, bool) {
	return m.keyMgmtList, m.keyMgmtListOk
}

// Dummy types to simulate concrete plugin implementations during the tests
type dummyCertIssuer struct {
	certificateissuer.CertificateIssuer
}
type dummyNotification struct{ notification.Notification }
type dummySysInfo struct {
	systeminformation.SystemInformation
}
type dummyIdentityMgmt struct {
	identitymanagement.IdentityManagement
}
type dummyKeystoreMgmt struct {
	keystoremanagement.KeystoreManagement
}
type dummyKeyMgmt struct{ keymanagement.KeyManagement }

func TestNewPluginCatalog(t *testing.T) {
	// Note: Because NewPluginCatalog relies on an external package function
	// (catalog.WrapAsPluginRepository), an integration test is typically best here.
	// This ensures the struct initializes without panicking.
	clg := &plugincatalog.Catalog{}

	// If WrapAsPluginRepository panics on a nil or empty catalog,
	// ensure clg is properly populated before this call.
	reg := NewPluginCatalog(clg)

	if reg == nil {
		t.Fatal("expected NewPluginCatalog to return a non-nil Registry")
	}
}

func TestRegistry_Singletons(t *testing.T) {
	// Setup our dummy instances
	expectedIssuer := &dummyCertIssuer{}
	expectedNotification := &dummyNotification{}
	expectedSysInfo := &dummySysInfo{}
	expectedIdentityMgmt := &dummyIdentityMgmt{}

	tests := []struct {
		name           string
		setupMock      func() *mockServiceAPIRegistry
		executeAndTest func(*testing.T, *Registry)
	}{
		{
			name: "CertificateIssuer - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{certIssuer: expectedIssuer, certIssuerOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.CertificateIssuer(); got != expectedIssuer {
					t.Errorf("CertificateIssuer() = %v, want %v", got, expectedIssuer)
				}
			},
		},
		{
			name: "CertificateIssuer - Not Found (Ignores ok)",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{certIssuer: nil, certIssuerOk: false}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.CertificateIssuer(); got != nil {
					t.Errorf("CertificateIssuer() = %v, want nil", got)
				}
			},
		},
		{
			name: "Notification - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{notification: expectedNotification, notificationOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.Notification(); got != expectedNotification {
					t.Errorf("Notification() = %v, want %v", got, expectedNotification)
				}
			},
		},
		{
			name: "SystemInformation - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{sysInfo: expectedSysInfo, sysInfoOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.SystemInformation(); got != expectedSysInfo {
					t.Errorf("SystemInformation() = %v, want %v", got, expectedSysInfo)
				}
			},
		},
		{
			name: "IdentityManagement - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{identityMgmt: expectedIdentityMgmt, identityMgmtOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.IdentityManagement(); got != expectedIdentityMgmt {
					t.Errorf("IdentityManagement() = %v, want %v", got, expectedIdentityMgmt)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()
			// Inject the mock directly into the struct to bypass NewPluginCatalog for unit testing
			reg := &Registry{Registry: mock}
			tt.executeAndTest(t, reg)
		})
	}
}

func TestRegistry_Collections(t *testing.T) {
	expectedKeystoreMap := map[string]keystoremanagement.KeystoreManagement{
		"aws": &dummyKeystoreMgmt{},
	}
	expectedKeystoreList := []keystoremanagement.KeystoreManagement{&dummyKeystoreMgmt{}}

	expectedKeyMgmtMap := map[string]keymanagement.KeyManagement{
		"gcp": &dummyKeyMgmt{},
	}
	expectedKeyMgmtList := []keymanagement.KeyManagement{&dummyKeyMgmt{}}

	tests := []struct {
		name           string
		setupMock      func() *mockServiceAPIRegistry
		executeAndTest func(*testing.T, *Registry)
	}{
		{
			name: "KeystoreManagements Map - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{keystoreMap: expectedKeystoreMap, keystoreMapOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				got := r.KeystoreManagements()
				if !reflect.DeepEqual(got, expectedKeystoreMap) {
					t.Errorf("KeystoreManagements() = %v, want %v", got, expectedKeystoreMap)
				}
			},
		},
		{
			name: "KeystoreManagementList - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{keystoreList: expectedKeystoreList, keystoreListOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				got := r.KeystoreManagementList()
				if !reflect.DeepEqual(got, expectedKeystoreList) {
					t.Errorf("KeystoreManagementList() = %v, want %v", got, expectedKeystoreList)
				}
			},
		},
		{
			name: "KeyManagements Map - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{keyMgmtMap: expectedKeyMgmtMap, keyMgmtMapOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				got := r.KeyManagements()
				if !reflect.DeepEqual(got, expectedKeyMgmtMap) {
					t.Errorf("KeyManagements() = %v, want %v", got, expectedKeyMgmtMap)
				}
			},
		},
		{
			name: "KeyManagementList - Found",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{keyMgmtList: expectedKeyMgmtList, keyMgmtListOk: true}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				got := r.KeyManagementList()
				if !reflect.DeepEqual(got, expectedKeyMgmtList) {
					t.Errorf("KeyManagementList() = %v, want %v", got, expectedKeyMgmtList)
				}
			},
		},
		{
			name: "KeyManagementList - Not Found (Returns nil slice)",
			setupMock: func() *mockServiceAPIRegistry {
				return &mockServiceAPIRegistry{keyMgmtList: nil, keyMgmtListOk: false}
			},
			executeAndTest: func(t *testing.T, r *Registry) {
				if got := r.KeyManagementList(); got != nil {
					t.Errorf("KeyManagementList() = %v, want nil", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()
			reg := &Registry{Registry: mock}
			tt.executeAndTest(t, reg)
		})
	}
}
