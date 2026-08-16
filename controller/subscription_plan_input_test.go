package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSubscriptionPlanInputNormalizesBenefitsOnlyQuota(t *testing.T) {
	plan := &model.SubscriptionPlan{
		TotalAmount:   -42,
		FiveHourLimit: 100,
		WeeklyLimit:   200,
		MonthlyLimit:  300,
	}

	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.EqualValues(t, -1, plan.TotalAmount)
	assert.Zero(t, plan.FiveHourLimit)
	assert.Zero(t, plan.WeeklyLimit)
	assert.Zero(t, plan.MonthlyLimit)
}

func TestNormalizeSubscriptionPlanInputRejectsInvalidUsageLimit(t *testing.T) {
	plan := &model.SubscriptionPlan{FiveHourLimit: int64(common.MaxQuota) + 1}

	require.Error(t, normalizeSubscriptionPlanInput(plan))
}

func TestNormalizeSubscriptionPlanInputRejectsInvalidRPMEntitlement(t *testing.T) {
	plan := &model.SubscriptionPlan{RateLimitGroups: `[{"group":"","rpm":120}]`}

	require.Error(t, normalizeSubscriptionPlanInput(plan))
}
