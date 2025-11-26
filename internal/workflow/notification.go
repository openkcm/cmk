package workflow

import "github.com/openkcm/cmk/internal/model"

func GetApproverUserNames(approvers []model.WorkflowApprover) []string {
	if len(approvers) == 0 {
		return []string{}
	}

	userNames := make([]string, 0, len(approvers))
	for _, approver := range approvers {
		if approver.UserName != "" {
			userNames = append(userNames, approver.UserName)
		}
	}

	return userNames
}

// GetNotificationRecipients returns the usernames to notify for a workflow transition.
func GetNotificationRecipients(workflow model.Workflow, transition Transition) []string {
	switch transition {
	case TransitionCreate:
		return GetApproverUserNames(workflow.Approvers)

	case TransitionApprove, TransitionReject:
		if workflow.InitiatorName != "" {
			return []string{workflow.InitiatorName}
		}

		return []string{}

	case TransitionConfirm, TransitionRevoke:
		decidedApprovers := make([]model.WorkflowApprover, 0)

		for _, approver := range workflow.Approvers {
			if approver.Approved.Valid {
				decidedApprovers = append(decidedApprovers, approver)
			}
		}

		return GetApproverUserNames(decidedApprovers)

	default:
		return []string{}
	}
}
