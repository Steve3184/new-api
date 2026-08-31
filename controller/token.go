package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type tokenAutoGroupsInput struct {
	Set    bool
	Groups []string
}

func (input *tokenAutoGroupsInput) UnmarshalJSON(data []byte) error {
	input.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		input.Groups = nil
		return nil
	}
	return common.Unmarshal(data, &input.Groups)
}

type tokenAutoRoutesInput struct {
	Set    bool
	Routes map[string][]string
}

func (input *tokenAutoRoutesInput) UnmarshalJSON(data []byte) error {
	input.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		input.Routes = nil
		return nil
	}
	return common.Unmarshal(data, &input.Routes)
}

type tokenRequest struct {
	model.Token
	AutoGroups tokenAutoGroupsInput `json:"auto_groups"`
	AutoRoutes tokenAutoRoutesInput `json:"auto_routes"`
}

type tokenResponse struct {
	*model.Token
	AutoGroups []string            `json:"auto_groups"`
	AutoRoutes map[string][]string `json:"auto_routes,omitempty"`
	TotalQuota int                 `json:"total_quota"`
	RPM        int                 `json:"rpm"`
}

const maxTokenRPMIDs = 100

func maxTokenQuota() int {
	quota, err := common.WalletQuotaFromDecimalStrict(
		decimal.NewFromInt(1_000_000_000).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	if err != nil {
		return common.MaxWalletQuota
	}
	return quota
}

func buildMaskedTokenResponseWithStats(token *model.Token, rpm int) *tokenResponse {
	if token == nil {
		return nil
	}
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	autoGroups, err := token.GetAutoGroups()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to parse auto groups for token %d: %v", token.Id, err))
		autoGroups = nil
	}
	if len(autoGroups) == 0 {
		autoGroups = nil
	}
	totalQuota := 0
	if !token.UnlimitedQuota {
		totalQuota = token.RemainQuota
		if token.UsedQuota > 0 {
			if token.UsedQuota > common.MaxQuota-totalQuota {
				totalQuota = common.MaxQuota
			} else {
				totalQuota += token.UsedQuota
			}
		}
	}
	// Virtual model routes are deliberately omitted from list and detail
	// responses. They are loaded through the token-scoped endpoint only when
	// the editor is opened, while RPM remains a one-time list snapshot.
	return &tokenResponse{Token: &maskedToken, AutoGroups: autoGroups, TotalQuota: totalQuota, RPM: rpm}
}

func buildMaskedTokenResponse(token *model.Token) *tokenResponse {
	return buildMaskedTokenResponseWithStats(token, 0)
}

func buildMaskedTokenResponses(tokens []*model.Token, rpms map[int]int) []*tokenResponse {
	maskedTokens := make([]*tokenResponse, 0, len(tokens))
	for _, token := range tokens {
		maskedTokens = append(maskedTokens, buildMaskedTokenResponseWithStats(token, rpms[token.Id]))
	}
	return maskedTokens
}

func getTokenRequestUserGroup(c *gin.Context) (string, error) {
	if userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup); userGroup != "" {
		return userGroup, nil
	}
	if userGroup := c.GetString("group"); userGroup != "" {
		return userGroup, nil
	}
	return model.GetUserGroup(c.GetInt("id"), false)
}

func setTokenAutoGroups(c *gin.Context, token *model.Token, groups []string) bool {
	if len(groups) == 0 {
		if err := token.SetAutoGroups(nil); err != nil {
			common.ApiError(c, err)
			return false
		}
		return true
	}

	maxCount := setting.GetMaxTokenAutoGroups()
	if len(groups) > maxCount {
		common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsTooMany, map[string]any{"Max": maxCount})
		return false
	}

	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, ok := seen[group]; ok {
			common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsDuplicate, map[string]any{"Group": group})
			return false
		}
		seen[group] = struct{}{}
		if !service.IsUserSelectableGroupForUser(c.GetInt("id"), userGroup, group) {
			common.ApiErrorI18n(c, i18n.MsgTokenAutoGroupsInvalid, map[string]any{"Group": group})
			return false
		}
	}

	if err := token.SetAutoGroups(groups); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

func setTokenAutoRoutes(c *gin.Context, token *model.Token, routes map[string][]string) bool {
	if len(routes) == 0 {
		if err := token.SetAutoRoutes(nil); err != nil {
			common.ApiError(c, err)
			return false
		}
		return true
	}
	if token.Group != "auto" {
		common.ApiError(c, fmt.Errorf("虚拟模型路由仅支持 auto 分组"))
		return false
	}
	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	autoGroups, err := token.GetAutoGroups()
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if len(autoGroups) == 0 {
		autoGroups = service.GetUserAutoGroupForUser(c.GetInt("id"), userGroup)
	}
	availableModels := make(map[string]struct{})
	for _, modelName := range service.GetGroupsEnabledModels(autoGroups) {
		availableModels[modelName] = struct{}{}
	}
	normalized := make(map[string][]string, len(routes))
	for virtualModel, chain := range routes {
		virtualModel = strings.TrimSpace(virtualModel)
		normalizedChain := make([]string, 0, len(chain))
		for _, modelName := range chain {
			normalizedChain = append(normalizedChain, strings.TrimSpace(modelName))
		}
		normalized[virtualModel] = normalizedChain
	}
	if err := model.ValidateAutoRoutes(normalized, availableModels); err != nil {
		common.ApiError(c, err)
		return false
	}
	if err := token.SetAutoRoutes(normalized); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(userId)
	rpms := make(map[int]int, len(tokens))
	tokenIDs := make([]int, 0, len(tokens))
	for _, token := range tokens {
		tokenIDs = append(tokenIDs, token.Id)
	}
	if rpmValues, rpmErr := model.GetTokenRPM(tokenIDs); rpmErr == nil {
		rpms = rpmValues
	} else {
		common.SysLog("failed to load token RPM: " + rpmErr.Error())
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens, rpms))
	common.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := common.GetPageQuery(c)

	tokens, total, err := model.SearchUserTokens(userId, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rpms := make(map[int]int, len(tokens))
	tokenIDs := make([]int, 0, len(tokens))
	for _, item := range tokens {
		tokenIDs = append(tokenIDs, item.Id)
	}
	if rpmValues, rpmErr := model.GetTokenRPM(tokenIDs); rpmErr == nil {
		rpms = rpmValues
	} else {
		common.SysLog("failed to load token RPM: " + rpmErr.Error())
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens, rpms))
	common.ApiSuccess(c, pageInfo)
}

func GetTokenRPM(c *gin.Context) {
	ids, err := parseTokenRPMIDs(c.QueryArray("ids"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rpms, err := model.GetUserTokenRPM(c.GetInt("id"), ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"rpms": rpms})
}

func parseTokenRPMIDs(values []string) ([]int, error) {
	ids := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		for _, rawID := range strings.Split(value, ",") {
			rawID = strings.TrimSpace(rawID)
			if rawID == "" {
				continue
			}
			id, err := strconv.Atoi(rawID)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("令牌 ID 无效: %s", rawID)
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			if len(ids) > maxTokenRPMIDs {
				return nil, fmt.Errorf("一次最多查询 %d 个令牌", maxTokenRPMIDs)
			}
		}
	}
	return ids, nil
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

// GetTokenAutoRoutes returns the virtual models configured on one API key.
// Keep this separate from the frequently refreshed key list because routes can
// be comparatively large and are only needed by the editor.
func GetTokenAutoRoutes(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("令牌 ID 无效"))
		return
	}
	token, err := model.GetTokenByIds(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	routes, err := token.GetAutoRoutes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if routes == nil {
		routes = map[string][]string{}
	}
	common.ApiSuccess(c, gin.H{"auto_routes": routes})
}

type tokenAutoRouteStatusResponse struct {
	VirtualModel string                            `json:"virtual_model"`
	Chain        []string                          `json:"chain"`
	Models       []model.TokenAutoRouteModelStatus `json:"models"`
}

func GetTokenAutoRouteStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Group != "auto" {
		common.ApiError(c, fmt.Errorf("虚拟模型路由仅支持 auto 分组"))
		return
	}
	routes, err := token.GetAutoRoutes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	autoGroups, err := token.GetAutoGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(autoGroups) == 0 {
		autoGroups = service.GetUserAutoGroupForUser(c.GetInt("id"), userGroup)
	}
	virtualModels := make([]string, 0, len(routes))
	for virtualModel := range routes {
		virtualModels = append(virtualModels, virtualModel)
	}
	sort.Strings(virtualModels)
	models := make([]string, 0)
	seenModels := make(map[string]struct{})
	for _, virtualModel := range virtualModels {
		for _, modelName := range routes[virtualModel] {
			if _, ok := seenModels[modelName]; ok {
				continue
			}
			seenModels[modelName] = struct{}{}
			models = append(models, modelName)
		}
	}
	modelStatuses, err := model.GetTokenAutoRouteModelStatuses(autoGroups, models)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	statusByModel := make(map[string]model.TokenAutoRouteModelStatus, len(modelStatuses))
	for _, status := range modelStatuses {
		statusByModel[status.Model] = status
	}
	response := make([]tokenAutoRouteStatusResponse, 0, len(virtualModels))
	for _, virtualModel := range virtualModels {
		chain := routes[virtualModel]
		chainStatuses := make([]model.TokenAutoRouteModelStatus, 0, len(chain))
		for _, modelName := range chain {
			if status, ok := statusByModel[modelName]; ok {
				chainStatuses = append(chainStatuses, status)
			}
		}
		response = append(response, tokenAutoRouteStatusResponse{
			VirtualModel: virtualModel,
			Chain:        chain,
			Models:       chainStatuses,
		})
	}
	common.ApiSuccess(c, gin.H{
		"routes":      response,
		"auto_groups": autoGroups,
		"updated_at":  common.GetTimestamp(),
	})
}

func ResetTokenUsedQuota(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.ResetTokenUsedQuota(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func GetTokenAutoGroups(c *gin.Context) {
	userGroup, err := getTokenRequestUserGroup(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"groups":    service.GetUserAutoGroupForUser(c.GetInt("id"), userGroup),
		"max_count": setting.GetMaxTokenAutoGroups(),
	})
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := model.GetTokenByKey(strings.TrimPrefix(tokenKey, "sk-"), false)
	if err != nil {
		common.SysError("failed to get token by key: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	request := tokenRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := request.Token
	// User API keys remain valid until explicitly reset; expiry is reserved for
	// server-managed channel tokens.
	token.ExpiredTime = -1
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	// 非无限额度时，检查额度值是否超出有效范围
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := maxTokenQuota()
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	// 检查用户令牌数量是否已达上限
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	if token.Group == "auto" {
		if !setTokenAutoGroups(c, &token, request.AutoGroups.Groups) {
			return
		}
		if !setTokenAutoRoutes(c, &token, request.AutoRoutes.Routes) {
			return
		}
	} else {
		token.CrossGroupRetry = false
		_ = token.SetAutoGroups(nil)
		_ = token.SetAutoRoutes(nil)
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := model.Token{
		UserId:             c.GetInt("id"),
		Name:               token.Name,
		Key:                key,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		AllowIps:           token.AllowIps,
		Group:              token.Group,
		CrossGroupRetry:    token.CrossGroupRetry,
		AutoGroups:         token.AutoGroups,
		AutoRoutes:         token.AutoRoutes,
	}
	err = cleanToken.Insert()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	statusOnly := c.Query("status_only")
	request := tokenRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token := request.Token
	token.ExpiredTime = -1
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := maxTokenQuota()
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status == common.TokenStatusEnabled {
		if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = token.Group
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
		if token.Group != "auto" {
			cleanToken.CrossGroupRetry = false
			_ = cleanToken.SetAutoGroups(nil)
			_ = cleanToken.SetAutoRoutes(nil)
		} else if request.AutoGroups.Set {
			if !setTokenAutoGroups(c, cleanToken, request.AutoGroups.Groups) {
				return
			}
			if request.AutoRoutes.Set && !setTokenAutoRoutes(c, cleanToken, request.AutoRoutes.Routes) {
				return
			}
		} else if request.AutoRoutes.Set {
			if !setTokenAutoRoutes(c, cleanToken, request.AutoRoutes.Routes) {
				return
			}
		}
	}
	err = cleanToken.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(cleanToken),
	})
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userId := c.GetInt("id")
	tokens, err := model.GetTokenKeysByIds(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keysMap := make(map[int]string)
	for _, t := range tokens {
		keysMap[t.Id] = t.GetFullKey()
	}
	common.ApiSuccess(c, gin.H{"keys": keysMap})
}
