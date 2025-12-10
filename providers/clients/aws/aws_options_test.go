package aws_test

import (
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/providers/clients/aws"
	"github.tools.sap/kms/cmk/providers/clients/aws/mock"
)

func TestOptions(t *testing.T) {
	tests := []struct {
		name     string
		optFunc  func(*kms.Options)
		expected func(*kms.Options) bool
	}{
		{
			name: "SetHTTPClient",
			optFunc: func(opts *kms.Options) {
				mockClient := &http.Client{}
				aws.SetHTTPClient(mockClient)(opts)
			},
			expected: func(opts *kms.Options) bool {
				return opts.HTTPClient != nil
			},
		},
		{
			name: "BaseEndpoint",
			optFunc: func(opts *kms.Options) {
				endpoint := "https://example.com"
				aws.BaseEndpoint(endpoint)(opts)
			},
			expected: func(opts *kms.Options) bool {
				return opts.BaseEndpoint != nil && *opts.BaseEndpoint == "https://example.com"
			},
		},
		{
			name: "SetEndpointResolver",
			optFunc: func(opts *kms.Options) {
				mockResolver := mock.EndpointResolver{}
				aws.SetEndpointResolver(mockResolver)(opts)
			},
			expected: func(opts *kms.Options) bool {
				return opts.EndpointResolverV2 != nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kmsOptions := &kms.Options{}
			tt.optFunc(kmsOptions)
			assert.True(t, tt.expected(kmsOptions), "%s should be set correctly", tt.name)
		})
	}
}
