package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUserGroupAccessAllowsAutoWhenCandidatesRemain(t *testing.T) {
	access := UserGroupAccess{
		UsableGroups: map[string]string{"default": "Default"},
		AutoGroups:   []string{"default"},
	}

	assert.True(t, access.Allows("auto"))
	assert.True(t, access.Allows("default"))
	assert.False(t, access.Allows("vip"))
}

func TestUserGroupAccessRejectsAutoWithoutUsableCandidates(t *testing.T) {
	access := UserGroupAccess{
		UsableGroups: map[string]string{"default": "Default"},
	}

	assert.False(t, access.Allows("auto"))
}

func TestFilterAutoGroupsPreservesOrderAndRemovesUnavailableDuplicates(t *testing.T) {
	usableGroups := map[string]string{
		"default": "Default",
		"vip":     "VIP",
	}

	groups := filterAutoGroups(
		[]string{"vip", "private", "default", "vip"},
		usableGroups,
	)

	assert.Equal(t, []string{"vip", "default"}, groups)
}

func TestGetRequestUserGroupAccessReusesCachedResolution(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	cached := UserGroupAccess{
		UsableGroups: map[string]string{"vip": "VIP"},
		AutoGroups:   []string{"vip"},
	}
	common.SetContextKey(c, constant.ContextKeyUserGroupAccess, cached)

	actual := GetRequestUserGroupAccess(c)

	assert.Equal(t, cached, actual)
}
