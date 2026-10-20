//go:build helmtests

package unit_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
	"gopkg.in/yaml.v3"

	configv1 "github.com/openkcm/plugin-sdk/proto/service/common/config/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/db/dsn"
	"github.com/openkcm/cmk/internal/featureflags"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/middleware"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/plugins/identity-management/scim"
	scimconfig "github.com/openkcm/cmk/internal/plugins/identity-management/scim/config"
)

// files holds the example files documented in charts/cmk/values.yaml. The tests below check
// that each one equals the documentation and parse it with the code of CMK that reads it.
const files = "testdata/files"

// documentedExample returns the example file content that values.yaml documents in the
// comment lines after the comment line ending in marker: the following lines indented two
// spaces deeper than that line's text, without the comment prefix.
func documentedExample(t *testing.T, marker string) string {
	t.Helper()
	data, err := os.ReadFile(chartPath + "/values.yaml")
	if err != nil {
		t.Fatalf("reading values.yaml: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasSuffix(line, marker) && strings.Contains(line, "#") {
			if start >= 0 {
				t.Fatalf("values.yaml has more than one comment line ending in %q", marker)
			}
			start = i
		}
	}
	if start < 0 {
		t.Fatalf("values.yaml has no comment line ending in %q", marker)
	}
	hash := strings.Index(lines[start], "#")
	text := lines[start][hash+2:]
	prefix := lines[start][:hash+2] + strings.Repeat(" ", len(text)-len(strings.TrimLeft(text, " "))+2)
	var example []string
	for _, line := range lines[start+1:] {
		if !strings.HasPrefix(line, prefix) {
			break
		}
		example = append(example, strings.TrimPrefix(line, prefix))
	}
	if len(example) == 0 {
		t.Fatalf("values.yaml has no example after %q", marker)
	}
	return strings.Join(example, "\n") + "\n"
}

// testdataFile returns the content of a file in testdata/files, and checks that it is the
// example documented after marker in values.yaml.
func testdataFile(t *testing.T, name, marker string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(files, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if documented := documentedExample(t, marker); string(data) != documented {
		t.Errorf("%s/%s differs from the example in values.yaml:\n%s", files, name, documented)
	}
	return string(data)
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err == nil {
		err = os.WriteFile(path, []byte(content), 0o600)
	}
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func fileRef(path string, format commoncfg.FileFormat, jsonPath string) commoncfg.SourceRef {
	return commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File:   commoncfg.CredentialFile{Path: path, Format: format, JSONPath: jsonPath},
	}
}

// TestFeatureFlagsFile loads the documented feature flags file with CMK's feature flag client,
// the way the CMK processes do at startup (featureflags.Init), and evaluates the flags the
// way CMK does: through the tenant configuration manager, without an evaluation context.
func TestFeatureFlagsFile(t *testing.T) {
	testdataFile(t, "flags.yaml", "/etc/featureflags/flags.yaml:")

	// The flag provider is global: CMK initializes it once per process.
	t.Cleanup(openfeature.Shutdown)
	load := func(t *testing.T, path string) *manager.TenantConfigManager {
		t.Helper()
		openfeature.Shutdown()
		cfg := &config.Config{}
		cfg.FeatureFlags = commoncfg.FeatureFlags{Enabled: true, FilePath: path, PollingInterval: time.Minute}
		err := featureflags.Init(cfg.FeatureFlags)
		if err != nil {
			t.Fatalf("CMK does not start with feature flags file %s: %v", path, err)
		}
		return manager.NewTenantConfigManager(nil, nil, cfg, nil, featureflags.NewClient())
	}

	t.Run("documented example", func(t *testing.T) {
		flags := load(t, filepath.Join(files, "flags.yaml"))
		// Plugin names as in config.plugins: CMK converts them to lower case.
		if !flags.IsHYOKAllowed(t.Context(), "AWS") {
			t.Error("enable_hyok_aws: HYOK keys in the AWS keystore are not allowed")
		}
		if !flags.IsKeyRotateDetectionEnabled(t.Context(), "AWS") {
			t.Error("detect_hyok_rotation_aws: key rotation detection is off")
		}
		if flags.IsHYOKAllowed(t.Context(), "GCP") {
			t.Error("enable_hyok_gcp is not in the file, but HYOK keys are allowed")
		}
		// CMK reads enable_byok_<default_keystore plugin> with the default value false; true
		// shows that the flag was found.
		byok, err := featureflags.NewClient().BooleanValue(t.Context(), "enable_byok_aws", true,
			openfeature.EvaluationContext{})
		if err != nil || byok {
			t.Errorf("enable_byok_aws: expected false, got %v (%v)", byok, err)
		}
	})

	t.Run("flags that are off", func(t *testing.T) {
		flags := load(t, filepath.Join(files, "flags-off.yaml"))
		if !flags.IsHYOKAllowed(t.Context(), "reference") {
			t.Fatal("enable_hyok_reference is off: the file was not read")
		}
		for _, plugin := range []string{"targeted", "rollout", "disabled", "invalid"} {
			if flags.IsHYOKAllowed(t.Context(), plugin) {
				t.Errorf("enable_hyok_%s is on", plugin)
			}
		}
	})

	// Last: after a failed start, the flag library cannot be initialized again.
	t.Run("not valid YAML stops the start", func(t *testing.T) {
		openfeature.Shutdown()
		path := writeFile(t, t.TempDir(), "flags.yaml", "enable_hyok_aws: [\n")
		err := featureflags.Init(commoncfg.FeatureFlags{Enabled: true, FilePath: path, PollingInterval: time.Minute})
		if err == nil {
			t.Error("CMK starts with a feature flags file that is not valid YAML")
		}
	})
}

// TestClientDataPublicKeys loads a directory of public keys the way the API server loads
// config.clientData.signingKeysPath (internal/daemon/server.go) and sends requests with
// client data signed with the private key through CMK's client data middleware.
func TestClientDataPublicKeys(t *testing.T) {
	signingKey := rsaKey(t)
	signingCert := newCertificate(t, "ExtAuthZ client data", signingKey, nil, false)
	dir := t.TempDir()
	writeFile(t, dir, "key1.pem", pemBlock("PUBLIC KEY", mustMarshalPKIX(t, &signingKey.PublicKey)))
	writeFile(t, dir, "key2.pem", pemBlock("RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&signingKey.PublicKey)))
	writeFile(t, dir, "key3.pem", pemBlock("CERTIFICATE", signingCert.Raw))
	writeFile(t, dir, "key4.pub", pemBlock("PUBLIC KEY", mustMarshalPKIX(t, &signingKey.PublicKey)))
	writeFile(t, dir, "sub/key5.pem", pemBlock("PUBLIC KEY", mustMarshalPKIX(t, &signingKey.PublicKey)))
	writeFile(t, dir, "key6.pem", pemBlock("PUBLIC KEY", mustMarshalPKIX(t, ecKey(t).Public())))
	writeFile(t, dir, "key7.pem", privateKeyPEM(t, signingKey, "PRIVATE KEY"))

	storage := keyvalue.NewMemoryStorage[string, []byte]()
	keys, err := loader.Create(
		loader.OnPath(dir),
		loader.WithExtension("pem"),
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(storage),
	)
	if err == nil {
		err = keys.Start()
	}
	if err != nil {
		t.Fatalf("loading the public keys: %v", err)
	}
	t.Cleanup(func() { _ = keys.Close() })

	handler := middleware.BusinessUserDataMiddleware(storage, []string{"issuer"}, nil)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	accepted := map[string]bool{
		"key1": true,  // PUBLIC KEY, as `openssl rsa -pubout` writes it
		"key2": true,  // RSA PUBLIC KEY
		"key3": true,  // CERTIFICATE with the public key
		"key4": false, // not a .pem file
		"key5": false, // in a subdirectory
		"key6": false, // not an RSA key
		"key7": false, // a private key
	}
	for keyID, expected := range accepted {
		clientData := &auth.ClientData{
			Identifier:         "alice",
			Groups:             []string{"KeyAdmin"},
			AuthContext:        map[string]string{"issuer": "https://idp.example.com"},
			KeyID:              keyID,
			SignatureAlgorithm: auth.SignatureAlgorithmRS256,
		}
		data, signature, err := clientData.Encode(signingKey)
		if err != nil {
			t.Fatalf("signing client data: %v", err)
		}
		request := httptest.NewRequest(http.MethodGet, "/cmk/v1/keys", nil)
		request.Header.Set(auth.HeaderClientData, data)
		request.Header.Set(auth.HeaderClientDataSignature, signature)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Code == http.StatusNoContent; got != expected {
			t.Errorf("key ID %s: expected accepted=%v, got status %d", keyID, expected, response.Code)
		}
	}

	_, err = loader.Create(loader.OnPath(filepath.Join(dir, "missing")))
	if err == nil || !strings.Contains(err.Error(), "path does not exist") {
		t.Errorf("expected `path does not exist` for a missing directory, got %v", err)
	}
}

// TestMTLSFiles loads PEM files of every documented kind with commoncfg.LoadMTLSConfig, which
// every mtls setting of CMK uses (connections, task queue, AMQP, audit, telemetry and the scim
// plugin), and makes a TLS connection with the result.
func TestMTLSFiles(t *testing.T) {
	root := newCertificate(t, "Example Root CA", rsaKey(t), nil, true)
	intermediate := newCertificate(t, "Example Intermediate CA", rsaKey(t), root, true)
	server := newCertificate(t, "server.example", rsaKey(t), root, false)

	keys := []struct {
		name   string
		key    crypto.Signer
		header string
	}{
		{"RSA PKCS #8", rsaKey(t), "PRIVATE KEY"},
		{"RSA PKCS #1", rsaKey(t), "RSA PRIVATE KEY"},
		{"ECDSA SEC 1", ecKey(t), "EC PRIVATE KEY"},
		{"ECDSA PKCS #8", ecKey(t), "PRIVATE KEY"},
		{"Ed25519 PKCS #8", ed25519Key(t), "PRIVATE KEY"},
	}
	for _, k := range keys {
		t.Run(k.name, func(t *testing.T) {
			client := newCertificate(t, "cmk", k.key, intermediate, false)
			dir := t.TempDir()
			// cert: the client certificate, then its intermediate CA.
			certFile := writeFile(t, dir, "tls.crt", pemBlock("CERTIFICATE", client.Raw)+pemBlock("CERTIFICATE", intermediate.Raw))
			keyFile := writeFile(t, dir, "tls.key", privateKeyPEM(t, k.key, k.header))
			caFile := writeFile(t, dir, "ca.crt", pemBlock("CERTIFICATE", root.Raw))
			serverCa := fileRef(caFile, "", "")
			tlsConfig, err := commoncfg.LoadMTLSConfig(&commoncfg.MTLS{
				Cert:       fileRef(certFile, commoncfg.BinaryFileFormat, ""),
				CertKey:    fileRef(keyFile, commoncfg.BinaryFileFormat, ""),
				ServerCA:   &serverCa,
				Attributes: &commoncfg.TLSAttributes{ServerName: "server.example"},
			})
			if err != nil {
				t.Fatalf("loading the mtls files: %v", err)
			}
			handshake(t, tlsConfig, server, root)
		})
	}

	// The JSON file of the event processor credentials in values-dev.yaml, from
	// env/blueprints/event-processor/secrets.json, with each value selected by jsonPath.
	t.Run("JSON file with jsonPath", func(t *testing.T) {
		var secrets map[string]string
		readJSON(t, "../../env/blueprints/event-processor/secrets.json", &secrets)
		key := rsaKey(t)
		client := newCertificate(t, "cmk", key, intermediate, false)
		for field, value := range map[string]string{
			"serverCA": pemBlock("CERTIFICATE", root.Raw),
			"cert":     pemBlock("CERTIFICATE", client.Raw) + pemBlock("CERTIFICATE", intermediate.Raw),
			"certKey":  privateKeyPEM(t, key, "PRIVATE KEY"),
		} {
			if _, ok := secrets[field]; !ok {
				t.Fatalf("secrets.json has no field %s", field)
			}
			secrets[field] = value
		}
		content, err := json.Marshal(secrets)
		if err != nil {
			t.Fatal(err)
		}
		path := writeFile(t, t.TempDir(), "secrets.json", string(content))
		serverCa := fileRef(path, commoncfg.JSONFileFormat, "$.serverCA")
		tlsConfig, err := commoncfg.LoadMTLSConfig(&commoncfg.MTLS{
			Cert:       fileRef(path, commoncfg.JSONFileFormat, "$.cert"),
			CertKey:    fileRef(path, commoncfg.JSONFileFormat, "$.certKey"),
			ServerCA:   &serverCa,
			Attributes: &commoncfg.TLSAttributes{ServerName: "server.example"},
		})
		if err != nil {
			t.Fatalf("loading the mtls values of secrets.json: %v", err)
		}
		handshake(t, tlsConfig, server, root)
	})
}

// TestValueReferenceFiles checks the documented rules for files of value references.
func TestValueReferenceFiles(t *testing.T) {
	dir := t.TempDir()
	password := writeFile(t, dir, "password", "secret\n")
	value, err := commoncfg.LoadValueFromSourceRef(fileRef(password, commoncfg.BinaryFileFormat, ""))
	if err != nil || string(value) != "secret\n" {
		t.Errorf("format binary: expected the whole file with its newline, got %q (%v)", value, err)
	}

	document := `{"password": "pw", "auth": {"cert": "c"}, "client-secret": "cs", "count": 1}`
	jsonFile := writeFile(t, dir, "credentials.json", document)
	for jsonPath, expected := range map[string]string{
		"$.password":      "pw",
		"$.auth.cert":     "c",
		"$.client-secret": "cs",
		"":                document,
	} {
		value, err := commoncfg.LoadValueFromSourceRef(fileRef(jsonFile, commoncfg.JSONFileFormat, jsonPath))
		if err != nil || string(value) != expected {
			t.Errorf("jsonPath %q: expected %q, got %q (%v)", jsonPath, expected, value, err)
		}
	}
	// Bracket notation is not supported, and the selected value must be a string.
	for _, jsonPath := range []string{"$['password']", "$.count", "$.auth"} {
		_, err := commoncfg.LoadValueFromSourceRef(fileRef(jsonFile, commoncfg.JSONFileFormat, jsonPath))
		if err == nil {
			t.Errorf("jsonPath %q: expected an error", jsonPath)
		}
	}
}

// TestDatabaseFiles builds the connection string of a database configuration with files the
// way CMK does (dsn.FromDBConfig) and parses it with the PostgreSQL driver, which reads the
// password, CA, certificate and key files.
func TestDatabaseFiles(t *testing.T) {
	root := newCertificate(t, "Example Root CA", rsaKey(t), nil, true)
	documented := documentedExample(t, "Example content of /etc/credentials/db.json:")

	for _, k := range []struct {
		name   string
		key    crypto.Signer
		header string
	}{
		{"RSA PKCS #8", rsaKey(t), "PRIVATE KEY"},
		{"RSA PKCS #1", rsaKey(t), "RSA PRIVATE KEY"},
		{"ECDSA SEC 1", ecKey(t), "EC PRIVATE KEY"},
	} {
		t.Run(k.name, func(t *testing.T) {
			dir := t.TempDir()
			client := newCertificate(t, "cmk", k.key, root, false)
			database := config.Database{
				Name: "cmk",
				Port: "5432",
				Host: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "postgres.example"},
				User: commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: "cmk"},
				Secret: fileRef(writeFile(t, dir, "database.json", documented),
					commoncfg.JSONFileFormat, "$.password"),
				Parameters: config.DBParameters{SSL: config.DBSSL{
					Mode:     "verify-full",
					RootCert: writeFile(t, dir, "database/ca.crt", pemBlock("CERTIFICATE", root.Raw)),
					Cert:     writeFile(t, dir, "database/tls.crt", pemBlock("CERTIFICATE", client.Raw)),
					Key:      writeFile(t, dir, "database/tls.key", privateKeyPEM(t, k.key, k.header)),
				}},
			}
			connString, err := dsn.FromDBConfig(database)
			if err != nil {
				t.Fatalf("building the connection string: %v", err)
			}
			parsed, err := pgconn.ParseConfig(connString)
			if err != nil {
				t.Fatalf("the PostgreSQL driver rejects the files: %v", err)
			}
			if parsed.Password != "<password>" || parsed.TLSConfig == nil ||
				len(parsed.TLSConfig.Certificates) != 1 || parsed.TLSConfig.RootCAs == nil {
				t.Errorf("unexpected connection settings: password %q, TLS %+v", parsed.Password, parsed.TLSConfig)
			}
		})
	}
}

// TestCryptoLayerAndSystemPropertiesFiles loads config.cryptoLayer.certX509Trusts and
// config.configurableContext from the documented files with CMK's configuration loader, and
// reads the crypto layer subjects the way CMK does (CertificateManager, data migration 00006).
func TestCryptoLayerAndSystemPropertiesFiles(t *testing.T) {
	testdataFile(t, "certX509Trusts.yaml", "which is also the whole content of the file with `source: file`:")
	trusts, err := filepath.Abs(filepath.Join(files, "certX509Trusts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	system, err := filepath.Abs(filepath.Join(files, "system.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// The system properties file holds the default value of config.configurableContext.
	values, err := os.ReadFile(chartPath + "/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var chart struct {
		Config struct {
			ConfigurableContext struct {
				Value string `yaml:"value"`
			} `yaml:"configurableContext"`
		} `yaml:"config"`
	}
	err = yaml.Unmarshal(values, &chart)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(system)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != chart.Config.ConfigurableContext.Value {
		t.Errorf("%s differs from config.configurableContext.value in values.yaml", system)
	}

	load := func(t *testing.T, trustsPath string) (*config.Config, error) {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, dir, "config.yaml", "cryptoLayer:\n"+
			"  certX509Trusts: { source: file, file: { path: "+trustsPath+", format: yaml } }\n"+
			"configurableContext: { source: file, file: { path: "+system+", format: yaml } }\n")
		return config.LoadConfig(commoncfg.WithPaths(dir))
	}

	cfg, err := load(t, trusts)
	if err != nil {
		t.Fatalf("CMK does not start with the files: %v", err)
	}
	properties := cfg.ContextModels.System
	if properties.Identifier.DisplayName != "GTID" || properties.Type.DisplayName != "Type" ||
		properties.OptionalProperties["externalName"].DisplayName != "Name" ||
		len(properties.OptionalProperties) != 3 {
		t.Errorf("unexpected system properties: %+v", properties)
	}

	data, err := commoncfg.LoadValueFromSourceRef(cfg.CryptoLayer.CertX509Trusts)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []*config.CryptoCert
	err = yaml.Unmarshal(data, &subjects)
	if err != nil || len(subjects) != 1 {
		t.Fatalf("expected one crypto layer subject, got %d (%v)", len(subjects), err)
	}
	certificate := model.NewClientCertificate(*subjects[0], "tenant-1")
	if certificate.Name != "eu10" || certificate.Subject.CommonName != "kms-crypto_tenant-1" ||
		!slices.Equal(certificate.Subject.Locality, []string{"eu10"}) ||
		!slices.Equal(certificate.Subject.OrganizationalUnit, []string{"KMS"}) {
		t.Errorf("unexpected crypto layer subject: %+v", certificate)
	}

	_, err = load(t, filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Error("CMK starts without the cryptoLayer.certX509Trusts file")
	}
}

// TestScimSettingsFile configures CMK's scim plugin, which takes the same settings as the scim
// binary of the cmk-full-plugins image, with every setting from the documented JSON file.
func TestScimSettingsFile(t *testing.T) {
	documented := testdataFile(t, "scim.json", "Example content of /etc/scim/scim.json:")
	settings, err := os.ReadFile(filepath.Join(files, "scim-settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if example := documentedExample(t, "`$.params.groupAttribute`, and so on. Example:"); !strings.Contains(string(settings), example) {
		t.Errorf("%s/scim-settings.yaml does not contain the example in values.yaml:\n%s", files, example)
	}

	root := newCertificate(t, "Example Root CA", rsaKey(t), nil, true)
	key := rsaKey(t)
	client := newCertificate(t, "cmk", key, root, false)
	content := documented
	for placeholder, value := range map[string]string{
		`"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----\n"`: pemBlock("CERTIFICATE", client.Raw),
		`"-----BEGIN PRIVATE KEY-----\n…\n-----END PRIVATE KEY-----\n"`: privateKeyPEM(t, key, "PRIVATE KEY"),
	} {
		quoted, err := json.Marshal(value)
		if err != nil || !strings.Contains(content, placeholder) {
			t.Fatalf("no placeholder %s in scim.json (%v)", placeholder, err)
		}
		content = strings.Replace(content, placeholder, string(quoted), 1)
	}
	path := writeFile(t, t.TempDir(), "scim.json", content)
	yamlConfiguration := strings.ReplaceAll(string(settings), "/etc/scim/scim.json", path)

	_, err = scim.NewPlugin().Configure(t.Context(), &configv1.ConfigureRequest{YamlConfiguration: yamlConfiguration})
	if err != nil {
		t.Fatalf("the scim plugin rejects the settings: %v", err)
	}
	var cfg scimconfig.Config
	decodeStrict(t, "scim", yamlConfiguration, &cfg)
	params, err := scim.CreateParams(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if params.BaseHost != "https://scim.example.com/scim/v2" || params.GroupAttribute != "displayName" ||
		params.UserAttribute != "groups.display" || params.ListMethod != "POST" {
		t.Errorf("unexpected settings: %+v", params)
	}

	// values-dev.yaml mounts env/blueprints/identity-management/scim.json, with the same fields.
	var blueprint, example any
	readJSON(t, "../../env/blueprints/identity-management/scim.json", &blueprint)
	err = json.Unmarshal([]byte(documented), &example)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(jsonFields(blueprint, ""), jsonFields(example, "")) {
		t.Errorf("the fields of the scim.json blueprint %v differ from the documented ones %v",
			jsonFields(blueprint, ""), jsonFields(example, ""))
	}

	// values-dev.yaml configures the scim plugin from that mounted file, selecting each field
	// with jsonPath in dot notation; common-sdk does not accept the bracket form.
	t.Run("values-dev.yaml", func(t *testing.T) {
		var dev struct {
			Config struct {
				Plugins []struct {
					Type              string `yaml:"type"`
					YamlConfiguration string `yaml:"yamlConfiguration"`
				} `yaml:"plugins"`
			} `yaml:"config"`
		}
		data, err := os.ReadFile(valuesDev)
		if err != nil {
			t.Fatal(err)
		}
		err = yaml.Unmarshal(data, &dev)
		if err != nil {
			t.Fatal(err)
		}
		var devSettings string
		for _, plugin := range dev.Config.Plugins {
			if plugin.Type == "IdentityManagementService" {
				devSettings = plugin.YamlConfiguration
			}
		}
		if devSettings == "" {
			t.Fatal("values-dev.yaml has no IdentityManagementService plugin settings")
		}
		devSettings = strings.ReplaceAll(devSettings, "/etc/credentials/identity-management/scim.json", path)

		_, err = scim.NewPlugin().Configure(t.Context(), &configv1.ConfigureRequest{YamlConfiguration: devSettings})
		if err != nil {
			t.Fatalf("the scim plugin rejects the settings of values-dev.yaml: %v", err)
		}
		var devCfg scimconfig.Config
		decodeStrict(t, "scim of values-dev.yaml", devSettings, &devCfg)
		devParams, err := scim.CreateParams(&devCfg)
		if err != nil {
			t.Fatal(err)
		}
		if devParams.BaseHost != "https://scim.example.com/scim/v2" || devParams.ListMethod != "POST" {
			t.Errorf("unexpected settings from values-dev.yaml: %+v", devParams)
		}
	})
}

// jsonFields returns the paths of the string values of a JSON document, such as $.auth.cert.
func jsonFields(value any, path string) []string {
	object, ok := value.(map[string]any)
	if !ok {
		return []string{"$" + path}
	}
	var result []string
	for key, child := range object {
		result = append(result, jsonFields(child, path+"."+key)...)
	}
	slices.Sort(result)
	return result
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, out)
	}
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
}

// handshake connects to a TLS server with the server certificate that requires a client
// certificate signed by root, with the client's tlsConfig.
func handshake(t *testing.T, tlsConfig *tls.Config, server, root *certificate) {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(root.Certificate)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	serverTLS := tls.Server(serverConn, &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{server.Raw}, PrivateKey: server.key}},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	})
	serverErr := make(chan error, 1)
	go func() { serverErr <- serverTLS.HandshakeContext(t.Context()) }()
	err := tls.Client(clientConn, tlsConfig).HandshakeContext(t.Context())
	if err != nil {
		t.Fatalf("TLS handshake of the client: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("TLS handshake of the server: %v", err)
	}
}

// certificate is a certificate generated for a test, with its private key.
type certificate struct {
	*x509.Certificate
	key crypto.Signer
}

// newCertificate creates a certificate for key, signed by parent or self-signed. A CA
// certificate can sign others; the others are valid for client and server authentication.
func newCertificate(t *testing.T, name string, key crypto.Signer, parent *certificate, isCA bool) *certificate {
	t.Helper()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	if isCA {
		template.IsCA = true
		template.BasicConstraintsValid = true
		template.KeyUsage = x509.KeyUsageCertSign
	} else {
		template.DNSNames = []string{name}
		template.KeyUsage = x509.KeyUsageDigitalSignature
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	}
	issuer, signer := template, key
	if parent != nil {
		issuer, signer = parent.Certificate, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, key.Public(), signer)
	if err != nil {
		t.Fatalf("creating the certificate %s: %v", name, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &certificate{Certificate: cert, key: key}
}

func rsaKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func ecKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func ed25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func pemBlock(header string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: header, Bytes: der}))
}

func mustMarshalPKIX(t *testing.T, key crypto.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// privateKeyPEM encodes key in the PEM block with the given header: PRIVATE KEY (PKCS #8),
// RSA PRIVATE KEY (PKCS #1) or EC PRIVATE KEY (SEC 1).
func privateKeyPEM(t *testing.T, key crypto.Signer, header string) string {
	t.Helper()
	var der []byte
	var err error
	switch header {
	case "PRIVATE KEY":
		der, err = x509.MarshalPKCS8PrivateKey(key)
	case "RSA PRIVATE KEY":
		der = x509.MarshalPKCS1PrivateKey(key.(*rsa.PrivateKey))
	case "EC PRIVATE KEY":
		der, err = x509.MarshalECPrivateKey(key.(*ecdsa.PrivateKey))
	}
	if err != nil || der == nil {
		t.Fatalf("encoding the private key as %s: %v", header, err)
	}
	return pemBlock(header, der)
}
