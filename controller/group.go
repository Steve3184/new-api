package controller

import (
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups, subscriptionGroups, err := service.GetUserGroupAccessSnapshot(userId, userGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if desc, ok := userUsableGroups[groupName]; ok {
			_, subscription := slices.BinarySearch(subscriptionGroups, groupName)
			ratio := 1.0
			if !subscription {
				if special, ok := ratio_setting.GetGroupGroupRatio(userGroup, groupName); ok {
					ratio = special
				} else {
					ratio = ratio_setting.GetGroupRatio(groupName)
				}
			}
			usableGroups[groupName] = map[string]interface{}{
				"ratio":         ratio,
				"desc":          desc,
				"default_model": ratio_setting.GetGroupDefaultModel(groupName),
				"subscription":  subscription,
			}
		}
	}
	if len(service.GetUserAutoGroupForUser(userId, userGroup)) > 0 {
		autoDescription := strings.TrimSpace(setting.AutoGroupDescription)
		if autoDescription == "" {
			autoDescription = setting.GetUsableGroupDescription("auto")
		}
		usableGroups["auto"] = map[string]any{
			"ratio":         "auto",
			"desc":          autoDescription,
			"default_model": ratio_setting.GetGroupDefaultModel("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
