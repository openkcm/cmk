package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// SetHTTPClient sets a custom HTTP client for the AWS SDK using kms.Options.
// It returns a function that applies the custom HTTP client to the kms.Options.
func SetHTTPClient(client kms.HTTPClient) func(*kms.Options) {
	return func(options *kms.Options) {
		options.HTTPClient = client
	}
}

// BaseEndpoint sets a base_client endpoint for the AWS SDK using kms.Options.
// It returns a function that applies the base_client endpoint to the kms.Options.
func BaseEndpoint(endpoint string) func(*kms.Options) {
	return func(options *kms.Options) {
		options.BaseEndpoint = &endpoint
	}
}

// SetEndpointResolver sets a custom endpoint resolver for the AWS SDK using kms.Options.
// It returns a function that applies the custom endpoint resolver to the kms.Options.
func SetEndpointResolver(endpointResolver kms.EndpointResolverV2) func(*kms.Options) {
	return func(options *kms.Options) {
		options.EndpointResolverV2 = endpointResolver
	}
}
