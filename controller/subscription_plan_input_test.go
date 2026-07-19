package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSubscriptionPlanInputNormalizesBenefitsOnlyQuota(t *testing.T) {
	plan := &model.SubscriptionPlan{TotalAmount: -42}

	require.NoError(t, normalizeSubscriptionPlanInput(plan))
	assert.EqualValues(t, -1, plan.TotalAmount)
}

func TestNormalizeSubscriptionPlanInputRejectsInvalidRPMEntitlement(t *testing.T) {
	plan := &model.SubscriptionPlan{RateLimitGroups: `[{"group":"","rpm":120}]`}

	require.Error(t, normalizeSubscriptionPlanInput(plan))
}
