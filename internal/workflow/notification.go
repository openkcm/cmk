package workflow

import (
	"context"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
)

func GetApproverUserNames(
	ctx context.Context,
	approvers []model.WorkflowApprover,
	idm identitymanagement.IdentityManagement,
) ([]string, error) {
	if len(approvers) == 0 {
		return []string{}, nil
	}

	userNames := make([]string, 0, len(approvers))
	for _, approver := range approvers {
		userName, err := approver.GetUserName(ctx, idm)
		if err != nil {
			return nil, err
		}
		if userName != "" {
			userNames = append(userNames, userName)
		}
	}

	return userNames, nil
}

// GetNotificationRecipients returns the usernames to notify for a workflow transition.
func GetNotificationRecipients(
	ctx context.Context,
	workflow model.Workflow,
	transition Transition,
	idm identitymanagement.IdentityManagement,
) ([]string, error) {
	switch transition {
	case TransitionCreate:
		return GetApproverUserNames(ctx, approverTasksOnly(workflow.Tasks), idm)

	case TransitionApprove, TransitionReject:
		initiatorName, err := workflow.GetInitiatorName(ctx, idm)
		if err != nil {
			return nil, err
		}
		if initiatorName != "" {
			return []string{initiatorName}, nil
		}

		return []string{}, nil

	case TransitionConfirm, TransitionRevoke:
		decidedApprovers := make([]model.WorkflowApprover, 0)

		for _, approver := range approverTasksOnly(workflow.Tasks) {
			if approver.Approved.Valid {
				decidedApprovers = append(decidedApprovers, approver)
			}
		}

		return GetApproverUserNames(ctx, decidedApprovers, idm)

	default:
		return []string{}, nil
	}
}

// approverTasksOnly returns only tasks with AssigneeRoleApprover.
func approverTasksOnly(tasks []model.WorkflowTask) []model.WorkflowTask {
	result := make([]model.WorkflowTask, 0, len(tasks))
	for _, t := range tasks {
		if t.AssigneeRole == model.AssigneeRoleApprover {
			result = append(result, t)
		}
	}
	return result
}
