package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/model"
)

func TestCloudIDFromAccessData(t *testing.T) {
	tests := map[string]struct {
		accessData model.KeystoreAccessData
		want       string
		wantErr    bool
	}{
		"aws role arn yields account id": {
			accessData: model.KeystoreAccessData{"roleArn": "arn:aws:iam::123456789012:role/kms"},
			want:       "123456789012",
		},
		"gcp project number": {
			accessData: model.KeystoreAccessData{"projectNumber": "987654321"},
			want:       "987654321",
		},
		"malformed role arn errors": {
			accessData: model.KeystoreAccessData{"roleArn": "not-an-arn"},
			wantErr:    true,
		},
		"empty account id in arn errors": {
			accessData: model.KeystoreAccessData{"roleArn": "arn:aws:iam:::role/kms"},
			wantErr:    true,
		},
		"no identifier present errors": {
			accessData: model.KeystoreAccessData{"other": "x"},
			wantErr:    true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := cloudIDFromAccessData(tc.accessData)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
