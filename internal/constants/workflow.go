package constants

// Workflow configuration bounds are security requirements, not operational
// preferences. Each limit represents the minimum acceptable security posture
// for a workflow-protected key management system:
//
//   - MinimumApprovals must be at least 2 to enforce dual control; at most 5
//     to remain operationally viable.
//   - RetentionPeriodDays must be at least 7 so completed and revoked workflows
//     remain auditable; at most 30 to bound storage growth.
//   - MaxExpiryPeriodDays is capped at 7 to prevent workflows from remaining
//     open indefinitely.
//
// These values are enforced at three independent layers (OpenAPI schema,
// manager validation, and database CHECK constraints) and are covered by
// security tests. Do not widen these bounds without a security review.
const (
	DefaultMinimumApprovalCount = 2
	MaxMinimumApprovals         = 5

	MinRetentionPeriodDays     = 7
	DefaultRetentionPeriodDays = 30
	MaxRetentionPeriodDays     = 30

	DefaultExpiryPeriodDays    = 7
	DefaultMaxExpiryPeriodDays = 7
)
