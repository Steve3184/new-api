package controller

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type tokenGroupMigrationRequest struct {
	SourceGroup string `json:"source_group"`
	TargetGroup string `json:"target_group"`
}

func GetTokenGroupNames(c *gin.Context) {
	sourceGroups, err := model.GetTokenGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groupRatios := ratio_setting.GetGroupRatioCopy()
	targetGroups := make([]string, 0, len(groupRatios)+1)
	for group := range groupRatios {
		targetGroups = append(targetGroups, group)
	}
	if len(setting.GetAutoGroups()) > 0 && !ratio_setting.ContainsGroupRatio("auto") {
		targetGroups = append(targetGroups, "auto")
	}
	sort.Strings(targetGroups)
	common.ApiSuccess(c, gin.H{
		"source_groups": sourceGroups,
		"target_groups": targetGroups,
	})
}

func MigrateTokenGroup(c *gin.Context) {
	var request tokenGroupMigrationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.SourceGroup = strings.TrimSpace(request.SourceGroup)
	request.TargetGroup = strings.TrimSpace(request.TargetGroup)
	validTarget := ratio_setting.ContainsGroupRatio(request.TargetGroup)
	if request.TargetGroup == "auto" {
		validTarget = len(setting.GetAutoGroups()) > 0
	}
	if !validTarget {
		common.ApiError(c, errors.New("target_group 不是当前可用分组"))
		return
	}
	count, err := model.MigrateTokenGroup(request.SourceGroup, request.TargetGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"source_group": request.SourceGroup,
		"target_group": request.TargetGroup,
		"migrated":     count,
	})
}
