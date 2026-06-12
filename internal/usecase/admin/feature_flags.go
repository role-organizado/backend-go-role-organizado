package admin

import (
	"context"
	"log/slog"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ListFeatureFlags implements portin.ListFeatureFlagsUseCase.
type ListFeatureFlags struct {
	repo portout.FeatureFlagRepository
}

// NewListFeatureFlags creates a new ListFeatureFlags use case.
func NewListFeatureFlags(r portout.FeatureFlagRepository) *ListFeatureFlags {
	return &ListFeatureFlags{repo: r}
}

// Execute lists all feature flags.
func (uc *ListFeatureFlags) Execute(ctx context.Context) ([]admin.FeatureFlag, error) {
	return uc.repo.FindAll(ctx)
}

// UpdateFeatureFlag implements portin.UpdateFeatureFlagUseCase.
type UpdateFeatureFlag struct {
	repo portout.FeatureFlagRepository
}

// NewUpdateFeatureFlag creates a new UpdateFeatureFlag use case.
func NewUpdateFeatureFlag(r portout.FeatureFlagRepository) *UpdateFeatureFlag {
	return &UpdateFeatureFlag{repo: r}
}

// Execute patches a feature flag by chave.
func (uc *UpdateFeatureFlag) Execute(ctx context.Context, chave string, upd admin.FeatureFlagUpdate) (*admin.FeatureFlag, error) {
	slog.InfoContext(ctx, "update feature flag", "chave", chave)
	return uc.repo.Update(ctx, chave, upd)
}
