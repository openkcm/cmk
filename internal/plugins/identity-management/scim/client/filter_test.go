package client_test

import (
	"testing"

	"github.com/openkcm/identity-management-plugins/pkg/clients/scim"
	"github.com/stretchr/testify/assert"
)

func TestFilterComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    scim.FilterExpression
		expected string
	}{
		{
			name: "Equal operator",
			input: scim.FilterComparison{
				Attribute: "name",
				Operator:  scim.FilterOperatorEqual,
				Value:     "John",
			},
			expected: `name eq "John"`,
		},
		{
			name: "Not Equal operator",
			input: scim.FilterComparison{
				Attribute: "type",
				Operator:  scim.FilterOperatorNotEqual,
				Value:     "employee",
			},
			expected: `type ne "employee"`,
		},
		{
			name: "Starts With operator",
			input: scim.FilterComparison{
				Attribute: "name",
				Operator:  scim.FilterOperatorStartsWith,
				Value:     "KMS",
			},
			expected: `name sw "KMS"`,
		}, {
			name: "Ends With operator",
			input: scim.FilterComparison{
				Attribute: "name",
				Operator:  scim.FilterOperatorEndsWith,
				Value:     "KMS",
			},
			expected: `name ew "KMS"`,
		},
		{
			name: "Negate expression",
			input: scim.FilterLogicalGroupNot{
				Expression: scim.FilterComparison{
					Attribute: "name",
					Operator:  scim.FilterOperatorEqual,
					Value:     "John",
				},
			},
			expected: `not name eq "John"`,
		},
		{
			name: "And Single expression",
			input: scim.FilterLogicalGroupAnd{
				Expressions: []scim.FilterExpression{
					scim.FilterComparison{
						Attribute: "name",
						Operator:  scim.FilterOperatorEqual,
						Value:     "John",
					},
				},
			},
			expected: `(name eq "John")`,
		},
		{
			name: "And Multiple expressions",
			input: scim.FilterLogicalGroupAnd{
				Expressions: []scim.FilterExpression{
					scim.FilterComparison{
						Attribute: "name",
						Operator:  scim.FilterOperatorEqual,
						Value:     "John",
					},
					scim.FilterComparison{
						Attribute: "group",
						Operator:  scim.FilterOperatorEqual,
						Value:     "CMK",
					},
				},
			},
			expected: `(name eq "John" and group eq "CMK")`,
		},
		{
			name: "Or Single expression",
			input: scim.FilterLogicalGroupOr{
				Expressions: []scim.FilterExpression{
					scim.FilterComparison{
						Attribute: "name",
						Operator:  scim.FilterOperatorEqual,
						Value:     "John",
					},
				},
			},
			expected: `(name eq "John")`,
		},
		{
			name: "Or Multiple expressions",
			input: scim.FilterLogicalGroupOr{
				Expressions: []scim.FilterExpression{
					scim.FilterComparison{
						Attribute: "name",
						Operator:  scim.FilterOperatorEqual,
						Value:     "John",
					},
					scim.FilterComparison{
						Attribute: "group",
						Operator:  scim.FilterOperatorEqual,
						Value:     "CMK",
					},
				},
			},
			expected: `(name eq "John" or group eq "CMK")`,
		},
		{
			name: "Combination expression",
			input: scim.FilterLogicalGroupAnd{
				Expressions: []scim.FilterExpression{
					scim.FilterComparison{
						Attribute: "name",
						Operator:  scim.FilterOperatorEqual,
						Value:     "John",
					},
					scim.FilterLogicalGroupOr{
						Expressions: []scim.FilterExpression{
							scim.FilterComparison{
								Attribute: "group",
								Operator:  scim.FilterOperatorEqual,
								Value:     "CMK",
							},
							scim.FilterComparison{
								Attribute: "type",
								Operator:  scim.FilterOperatorEqual,
								Value:     "employee",
							},
						},
					},
				},
			},
			expected: `(name eq "John" and (group eq "CMK" or type eq "employee"))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToString()
			assert.Equal(t, tt.expected, result)
		})
	}
}
