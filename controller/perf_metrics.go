package controller

import (
	"net/http"
	"slices"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	recentModels := c.QueryArray("recent_model")
	if len(recentModels) > perfmetrics.MaxRecentModels {
		common.ApiErrorStatus(c, http.StatusBadRequest, common.NewMessage("Invalid parameters"))
		return
	}
	activeGroups := append(lo.Keys(ratio_setting.GetGroupRatioCopy()), "auto")
	result, err := perfmetrics.QuerySummaryAll(hours, activeGroups, recentModels...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetrics(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	result, err := perfmetrics.Query(perfmetrics.QueryParams{
		Model:         modelName,
		Group:         c.Query("group"),
		Hours:         hours,
		AllowedGroups: append(lo.Keys(ratio_setting.GetGroupRatioCopy()), "auto"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
func GetPerfMetricsStatus(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	activeGroups := statusCheckActiveGroups()
	var cacheExcludedModels []string
	_ = common.UnmarshalJsonStr(common.StatusCheckCacheExcludedModels, &cacheExcludedModels)
	result, err := perfmetrics.QueryStatus(hours, activeGroups, cacheExcludedModels)
	if err == nil {
		err = perfmetrics.AttachStatusCheckDegradation(&result, hours)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func statusCheckActiveGroups() []string {
	activeRatios := ratio_setting.GetGroupRatioCopy()
	activeGroups := make([]string, 0, len(activeRatios)+1)
	var configuredGroups []string
	_ = common.UnmarshalJsonStr(common.StatusCheckGroups, &configuredGroups)
	if len(configuredGroups) > 0 {
		seen := make(map[string]struct{}, len(configuredGroups))
		for _, group := range configuredGroups {
			if _, exists := seen[group]; exists {
				continue
			}
			if _, active := activeRatios[group]; !active && group != "auto" {
				continue
			}
			seen[group] = struct{}{}
			activeGroups = append(activeGroups, group)
		}
		return activeGroups
	}
	activeGroups = append(lo.Keys(activeRatios), "auto")
	sort.Strings(activeGroups)
	return activeGroups
}

type statusCheckFlexibleProbeGroup struct {
	Group  string
	Config perfmetrics.StatusCheckFlexibleGroupConfig
}

// statusCheckFlexibleProbeGroups deliberately requires both an explicit status
// group list and an enabled per-group configuration. An empty visible-group
// list means "show all" but must never turn every group into a billable active
// probe target. Groups with degradation checks are skipped: those checks
// already fill idle hours.
func statusCheckFlexibleProbeGroups() []statusCheckFlexibleProbeGroup {
	flexibleConfig := perfmetrics.GetStatusCheckFlexibleConfig()
	degradationConfig := perfmetrics.GetStatusCheckDegradationConfig()
	groups := make([]statusCheckFlexibleProbeGroup, 0)
	for _, group := range statusCheckExplicitProbeGroups() {
		groupConfig, enabled := flexibleConfig.EnabledGroup(group)
		if !enabled {
			continue
		}
		if _, degradationEnabled := degradationConfig.EnabledGroup(group); degradationEnabled {
			continue
		}
		groups = append(groups, statusCheckFlexibleProbeGroup{
			Group:  group,
			Config: groupConfig,
		})
	}
	return groups
}

// statusCheckExplicitProbeGroups returns the explicitly configured, active,
// non-auto status groups: the only groups that may receive billable active
// requests from the status page.
func statusCheckExplicitProbeGroups() []string {
	var configuredGroups []string
	if err := common.UnmarshalJsonStr(common.StatusCheckGroups, &configuredGroups); err != nil || len(configuredGroups) == 0 {
		return nil
	}
	activeRatios := ratio_setting.GetGroupRatioCopy()
	groups := make([]string, 0, len(configuredGroups))
	for _, group := range configuredGroups {
		if group == "auto" || slices.Contains(groups, group) {
			continue
		}
		if _, active := activeRatios[group]; !active {
			continue
		}
		groups = append(groups, group)
	}
	return groups
}

func filterActiveGroups(groups []perfmetrics.GroupResult) []perfmetrics.GroupResult {
	activeRatios := ratio_setting.GetGroupRatioCopy()
	return lo.Filter(groups, func(g perfmetrics.GroupResult, _ int) bool {
		_, ok := activeRatios[g.Group]
		return ok || g.Group == "auto"
	})
}
