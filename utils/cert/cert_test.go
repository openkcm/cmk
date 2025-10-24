package cert_test

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/utils/cert"
	"github.com/openkcm/cmk/utils/cert/mock"
)

var (
	ErrMock = errors.New("mock error")
)

// TestGenerateTemporaryCertAndKey tests the GenerateTemporaryCertAndKey function
func TestGenerateTemporaryCertAndKey(t *testing.T) {
	tests := []struct {
		name          string
		certCreator   cert.CertificateCreator
		pemEncoder    cert.PEMEncoder
		randReader    io.Reader
		expectedError error
	}{
		{
			name:          "Success",
			certCreator:   &cert.DefaultCertCreator{},
			pemEncoder:    &cert.DefaultPEMEncoder{},
			randReader:    rand.Reader,
			expectedError: nil,
		},
		{
			name: "FailureToCreateCertificate",
			certCreator: &mock.CertCreator{
				ShouldReturnErrorForCert: true,
			},
			pemEncoder:    &cert.DefaultPEMEncoder{},
			randReader:    rand.Reader,
			expectedError: cert.ErrFailedToCreateCertificate,
		},
		{
			name: "FailureToMarshalPrivateKey",
			certCreator: &mock.CertCreator{
				ShouldReturnErrorForKey: true,
			},
			pemEncoder:    &cert.DefaultPEMEncoder{},
			randReader:    rand.Reader,
			expectedError: cert.ErrFailedToMarshalPrivateKey,
		},
		{
			name:          "FailureToGeneratePrivateKey",
			certCreator:   &cert.DefaultCertCreator{},
			pemEncoder:    &cert.DefaultPEMEncoder{},
			randReader:    &errorReader{},
			expectedError: cert.ErrFailedToGeneratePrivateKey,
		},
		{
			name:          "FailureToEncodePEM",
			certCreator:   &cert.DefaultCertCreator{},
			pemEncoder:    &mock.PEMEncoder{ShouldReturnError: 0},
			randReader:    rand.Reader,
			expectedError: cert.ErrFailedToWriteDataToCert,
		},
		{
			name:          "FailureToEncodePEM",
			certCreator:   &cert.DefaultCertCreator{},
			pemEncoder:    &mock.PEMEncoder{ShouldReturnError: 1},
			randReader:    rand.Reader,
			expectedError: cert.ErrFailedToWriteDataToKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalRandReader := rand.Reader
			rand.Reader = tt.randReader

			certFile, keyFile, err := cert.ExportGenerateTemporaryCertAndKeyWithSettings()(
				tt.certCreator,
				tt.pemEncoder,
			)
			if tt.expectedError != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectedError)
				assert.NoFileExists(t, certFile)
				assert.NoFileExists(t, keyFile)
			} else {
				require.NoError(t, err)
				assert.FileExists(t, certFile)
				assert.FileExists(t, keyFile)
				cleanupFiles(t, certFile, keyFile)
			}

			rand.Reader = originalRandReader
		})
	}
}

// cleanupFiles removes the given files
func cleanupFiles(t *testing.T, files ...string) {
	t.Helper()

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			t.Errorf("failed to remove file %s: %v", file, err)
		}
	}
}

// errorReader is an io.Reader that always returns an error
type errorReader struct{}

// Read always returns an error
func (e *errorReader) Read([]byte) (int, error) {
	return 0, ErrMock
}
