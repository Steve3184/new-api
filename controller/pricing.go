package controller

import (
	"maps"
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		// The pricing cache is shared by all viewers.
		groups := make([]string, 0, len(item.EnableGroup))
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok || group == "all" {
				groups = append(groups, group)
			}
		}
		if len(groups) > 0 {
			item.EnableGroup = groups
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// modelSquareGroups are the groups the model square shows one viewer: the
// groups that viewer can use, the groups an administrator made visible to
// everyone, and the subscription groups on sale. Pricing and performance
// metrics read the same set, so neither exposes a group the other hides.
type modelSquareGroups struct {
	usable       map[string]string
	visible      []string
	subscription []string
	descriptions map[string]string
	// shown maps every group above to the description the viewer sees.
	shown map[string]string
}

func resolveModelSquareGroups(userID int, userGroup string) (modelSquareGroups, error) {
	usable, subscription, err := service.GetUserGroupAccessSnapshot(userID, userGroup)
	if err != nil {
		return modelSquareGroups{}, err
	}
	visible := slices.Clone(console_setting.GetModelSquareVisibleGroups())
	subscription = slices.DeleteFunc(subscription, func(name string) bool {
		return !ratio_setting.ContainsGroupRatio(name)
	})
	for _, name := range subscription {
		if !slices.Contains(visible, name) {
			visible = append(visible, name)
		}
	}
	shown := make(map[string]string, len(usable)+len(visible))
	maps.Copy(shown, usable)
	descriptions := console_setting.GetModelSquareVisibleGroupDescriptions()
	for _, name := range visible {
		if _, exists := shown[name]; exists {
			continue
		}
		if description := descriptions[name]; description != "" {
			shown[name] = description
		} else {
			shown[name] = setting.GetUsableGroupDescription(name)
		}
	}
	return modelSquareGroups{
		usable:       usable,
		visible:      visible,
		subscription: subscription,
		descriptions: descriptions,
		shown:        shown,
	}, nil
}

func GetPricing(c *gin.Context) {
	userID := c.GetInt("id")
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups, err := resolveModelSquareGroups(userID, userGroup)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pricing := filterPricingByUsableGroups(model.GetPricing(), groups.shown)
	groupCategories := console_setting.GetModelSquareGroupCategories()
	maps.DeleteFunc(groupCategories, func(name, _ string) bool {
		_, ok := groups.shown[name]
		return !ok
	})
	groupRatio := ratio_setting.GetGroupRatioCopy()
	for group := range groupRatio {
		if _, ok := groups.shown[group]; !ok {
			delete(groupRatio, group)
			continue
		}
		if userGroup != "" {
			if ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group); ok {
				groupRatio[group] = ratio
			}
		}
	}
	// Subscription groups are paid for by the plan, not by a group ratio.
	for _, name := range groups.subscription {
		groupRatio[name] = 1
	}

	c.JSON(200, gin.H{
		"success":                         true,
		"data":                            pricing,
		"vendors":                         model.GetVendors(),
		"group_ratio":                     groupRatio,
		"usable_group":                    groups.usable,
		"model_square_groups":             groups.visible,
		"subscription_groups":             groups.subscription,
		"model_square_group_descriptions": groups.descriptions,
		"model_square_group_categories":   groupCategories,
		"supported_endpoint":              model.GetSupportedEndpointMap(),
		"auto_groups":                     service.GetUserAutoGroupForUser(userID, userGroup),
		"pricing_version":                 "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccessT(c, "Model ratios reset", nil)
}
