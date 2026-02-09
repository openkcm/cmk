package client_test

import (
	"testing"

	"github.com/openkcm/identity-management-plugins/pkg/clients/scim"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/plugins/identity-management/scim/client"
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

func TestNullFilterExpression(t *testing.T) {
	f := client.NullFilterExpression{}
	if got := f.ToString(); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestFilterComparison_ToString(t *testing.T) {
	tests := []struct {
		name     string
		filter   client.FilterComparison
		expected string
	}{
		{
			name: "equal operator",
			filter: client.FilterComparison{
				Attribute: "userName",
				Operator:  client.FilterOperatorEqual,
				Value:     "john",
			},
			expected: `userName eq "john"`,
		},
		{
			name: "case insensitive equal operator",
			filter: client.FilterComparison{
				Attribute: "email",
				Operator:  client.FilterOperatorEqualCI,
				Value:     "TEST@EXAMPLE.COM",
			},
			expected: `email eq_ci "TEST@EXAMPLE.COM"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.ToString(); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFilterLogicalGroupAnd_ToString(t *testing.T) {
	filter := client.FilterLogicalGroupAnd{
		Expressions: []client.FilterExpression{
			client.FilterComparison{
				Attribute: "userName",
				Operator:  client.FilterOperatorEqual,
				Value:     "john",
			},
			client.FilterComparison{
				Attribute: "active",
				Operator:  client.FilterOperatorEqual,
				Value:     "true",
			},
		},
	}

	expected := `(userName eq "john" and active eq "true")`
	if got := filter.ToString(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestFilterLogicalGroupOr_ToString(t *testing.T) {
	filter := client.FilterLogicalGroupOr{
		Expressions: []client.FilterExpression{
			client.FilterComparison{
				Attribute: "role",
				Operator:  client.FilterOperatorEqual,
				Value:     "admin",
			},
			client.FilterComparison{
				Attribute: "role",
				Operator:  client.FilterOperatorEqual,
				Value:     "user",
			},
		},
	}

	expected := `(role eq "admin" or role eq "user")`
	if got := filter.ToString(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestFilterLogicalGroupNot_ToString(t *testing.T) {
	filter := client.FilterLogicalGroupNot{
		Expression: client.FilterComparison{
			Attribute: "active",
			Operator:  client.FilterOperatorEqual,
			Value:     "false",
		},
	}

	expected := `not active eq "false"`
	if got := filter.ToString(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestNestedLogicalGroups(t *testing.T) {
	filter := client.FilterLogicalGroupAnd{
		Expressions: []client.FilterExpression{
			client.FilterLogicalGroupOr{
				Expressions: []client.FilterExpression{
					client.FilterComparison{
						Attribute: "role",
						Operator:  client.FilterOperatorEqual,
						Value:     "admin",
					},
					client.FilterComparison{
						Attribute: "role",
						Operator:  client.FilterOperatorEqual,
						Value:     "user",
					},
				},
			},
			client.FilterLogicalGroupNot{
				Expression: client.FilterComparison{
					Attribute: "active",
					Operator:  client.FilterOperatorEqual,
					Value:     "false",
				},
			},
		},
	}

	expected := `((role eq "admin" or role eq "user") and not active eq "false")`
	if got := filter.ToString(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
