package admin

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// CountPendingApprovals implements portin.CountPendingApprovalsUseCase.
type CountPendingApprovals struct {
	repo portout.ApprovalRepository
}

// NewCountPendingApprovals creates a new CountPendingApprovals use case.
func NewCountPendingApprovals(r portout.ApprovalRepository) *CountPendingApprovals {
	return &CountPendingApprovals{repo: r}
}

// Execute counts PENDING approval items for the approver. On error it returns 0
// (Java's defensive behaviour).
func (uc *CountPendingApprovals) Execute(ctx context.Context, approverID string) (int64, error) {
	n, err := uc.repo.CountPending(ctx, approverID)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// ListPendingApprovals implements portin.ListPendingApprovalsUseCase.
type ListPendingApprovals struct {
	repo portout.ApprovalRepository
}

// NewListPendingApprovals creates a new ListPendingApprovals use case.
func NewListPendingApprovals(r portout.ApprovalRepository) *ListPendingApprovals {
	return &ListPendingApprovals{repo: r}
}

// Execute lists PENDING approval items for the approver.
func (uc *ListPendingApprovals) Execute(ctx context.Context, approverID string) ([]admin.ApprovalItem, error) {
	items, err := uc.repo.FindPending(ctx, approverID)
	if err != nil {
		return []admin.ApprovalItem{}, nil
	}
	return items, nil
}

// ListApprovalHistory implements portin.ListApprovalHistoryUseCase.
type ListApprovalHistory struct {
	repo portout.ApprovalRepository
}

// NewListApprovalHistory creates a new ListApprovalHistory use case.
func NewListApprovalHistory(r portout.ApprovalRepository) *ListApprovalHistory {
	return &ListApprovalHistory{repo: r}
}

// Execute lists resolved approval items for the approver.
func (uc *ListApprovalHistory) Execute(ctx context.Context, approverID string) ([]admin.ApprovalItem, error) {
	items, err := uc.repo.FindHistory(ctx, approverID)
	if err != nil {
		return []admin.ApprovalItem{}, nil
	}
	return items, nil
}
