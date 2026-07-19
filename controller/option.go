package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/playground_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isPublicSiteKey := strings.HasSuffix(k, "SiteKey")
		isSensitiveKey := !isPublicSiteKey && (strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key"))
		if isSensitiveKey {
			// Return a sentinel instead of omitting the key entirely. The
			// frontend uses this to distinguish "already configured" from
			// "never set": a non-empty sentinel passes validation without
			// revealing the actual secret, and the save loop skips it when
			// the user has not typed a new value.
			sentinel := ""
			if value != "" {
				sentinel = "***"
			}
			options = append(options, &model.Option{Key: k, Value: sentinel})
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type customTabOption struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
	External bool   `json:"external"`
}

func isValidCustomTabURL(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") {
		return true
	}
	parsedURL, err := url.ParseRequestURI(value)
	return err == nil &&
		(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") &&
		parsedURL.Host != ""
}

func syncCapDifficultyForOption(c *gin.Context, key, value string) error {
	switch key {
	case "CapServerURL", "CapAdminAPIKey", "CapSiteKey", "CapCheckinSiteKey", "LoginCaptchaDifficulty", "CheckinCaptchaDifficulty":
	default:
		return nil
	}

	serverURL := common.CapServerURL
	apiKey := common.CapAdminAPIKey
	loginSiteKey := common.CapSiteKey
	checkinSiteKey := common.CapCheckinSiteKey
	loginDifficulty := common.LoginCaptchaDifficulty
	checkinDifficulty := common.CheckinCaptchaDifficulty

	switch key {
	case "CapServerURL":
		serverURL = strings.TrimRight(strings.TrimSpace(value), "/")
	case "CapAdminAPIKey":
		apiKey = value
	case "CapSiteKey":
		loginSiteKey = value
	case "CapCheckinSiteKey":
		checkinSiteKey = value
	case "LoginCaptchaDifficulty":
		loginDifficulty, _ = strconv.Atoi(value)
	case "CheckinCaptchaDifficulty":
		checkinDifficulty, _ = strconv.Atoi(value)
	}

	if loginSiteKey != "" && loginSiteKey == checkinSiteKey && loginDifficulty != checkinDifficulty {
		return fmt.Errorf("login and check-in require different Cap site keys when their difficulties differ")
	}

	switch key {
	case "CapCheckinSiteKey", "CheckinCaptchaDifficulty":
		return service.SyncCapDifficulty(c.Request.Context(), serverURL, apiKey, checkinSiteKey, checkinDifficulty)
	case "CapSiteKey", "LoginCaptchaDifficulty":
		return service.SyncCapDifficulty(c.Request.Context(), serverURL, apiKey, loginSiteKey, loginDifficulty)
	default:
		if err := service.SyncCapDifficulty(c.Request.Context(), serverURL, apiKey, loginSiteKey, loginDifficulty); err != nil {
			return err
		}
		return service.SyncCapDifficulty(c.Request.Context(), serverURL, apiKey, checkinSiteKey, checkinDifficulty)
	}
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	// Reject the read-only sentinel that GetOptions emits for already-set
	// sensitive fields. The frontend skips unchanged password fields, but
	// guard here as well so a stale client can never accidentally overwrite
	// a real secret with the placeholder.
	if option.Value == "***" {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
		return
	}
	switch option.Key {
	case "CaptchaType":
		if option.Value != "turnstile" && option.Value != "hcaptcha" && option.Value != "cap" {
			common.ApiErrorMsg(c, "CaptchaType must be turnstile, hcaptcha, or cap")
			return
		}
	case "checkin_setting.min_user_quota":
		quota, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || quota < 0 {
			common.ApiErrorMsg(c, "Check-in minimum user quota must be a non-negative integer")
			return
		}
	case "CapServerURL":
		if option.Value != "" {
			parsedURL, parseErr := url.ParseRequestURI(option.Value.(string))
			if parseErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
				common.ApiErrorMsg(c, "Cap server URL must be a valid HTTP or HTTPS URL")
				return
			}
		}
	case "WorkerMeshyImageProxyBaseURL":
		if option.Value != "" {
			parsedURL, parseErr := url.ParseRequestURI(strings.TrimSpace(option.Value.(string)))
			if parseErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
				common.ApiErrorMsg(c, "Meshy image proxy base URL must be a valid HTTP or HTTPS URL")
				return
			}
		}
	case "WorkerMeshyImageProxyEnabled":
		if option.Value == "true" && (strings.TrimSpace(system_setting.WorkerMeshyImageProxyBaseURL) == "" || strings.TrimSpace(system_setting.WorkerMeshyImageProxyAPIKey) == "") {
			common.ApiErrorMsg(c, "Configure the Meshy image proxy base URL and API key before enabling it")
			return
		}
	case "LoginCaptchaDifficulty", "CheckinCaptchaDifficulty":
		difficulty, parseErr := strconv.Atoi(option.Value.(string))
		if parseErr != nil || difficulty < 1 || difficulty > 8 {
			common.ApiErrorMsg(c, "Cap difficulty must be between 1 and 8")
			return
		}
	case "PaymentAnnouncement":
		if len(option.Value.(string)) > 50000 {
			common.ApiErrorMsg(c, "Payment announcement is too long")
			return
		}
	case "StatusCheckAnnouncement":
		if len(option.Value.(string)) > 50000 {
			common.ApiErrorMsg(c, "Status check announcement is too long")
			return
		}
	case "CustomTabs":
		var tabs []customTabOption
		if err = common.UnmarshalJsonStr(option.Value.(string), &tabs); err != nil || len(tabs) > 50 {
			common.ApiErrorMsg(c, "Invalid custom tabs configuration")
			return
		}
		validCategories := map[string]bool{"chat": true, "general": true, "personal": true, "admin": true}
		for _, tab := range tabs {
			if strings.TrimSpace(tab.ID) == "" || strings.TrimSpace(tab.Label) == "" || len(tab.Label) > 80 || len(tab.URL) > 2048 || len(tab.Icon) > 64 || !validCategories[tab.Category] {
				common.ApiErrorMsg(c, "Invalid custom tab entry")
				return
			}
			if !isValidCustomTabURL(tab.URL) {
				common.ApiErrorMsg(c, "Custom tab URLs must start with / or use HTTP or HTTPS")
				return
			}
		}
	case "StatusCheckGroups", "StatusCheckCacheExcludedModels":
		var values []string
		if err = common.UnmarshalJsonStr(option.Value.(string), &values); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "status check configuration must be a JSON string array",
			})
			return
		}
		maxEntries := 500
		if option.Key == "StatusCheckGroups" {
			maxEntries = 100
		}
		if len(values) > maxEntries {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("status check configuration cannot exceed %d entries", maxEntries),
			})
			return
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len(value) > 256 {
				common.ApiErrorMsg(c, "status check entries must be non-empty and at most 256 characters")
				return
			}
		}
	case "PlaygroundSettings":
		if err = playground_setting.ValidateJSONString(option.Value.(string)); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value.(string)) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
	default:
		if isPaymentComplianceOptionKey(option.Key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "HCaptchaEnabled":
		if option.Value == "true" && (common.HCaptchaSiteKey == "" || common.HCaptchaSecretKey == "") {
			common.ApiErrorMsg(c, "Configure the hCaptcha site key and secret before enabling hCaptcha")
			return
		}
	case "CapEnabled":
		if option.Value == "true" && (common.CapServerURL == "" || common.CapSiteKey == "" || common.CapSecretKey == "" || common.CapAdminAPIKey == "") {
			common.ApiErrorMsg(c, "Configure the Cap server, API key, login site key, and secret before enabling Cap")
			return
		}
	case "ForceCheckinCaptcha":
		if option.Value == "true" {
			if common.CaptchaType == "cap" && (!common.CapEnabled || common.CapCheckinSiteKey == "" || common.CapCheckinSecretKey == "") {
				common.ApiErrorMsg(c, "Configure and enable the Cap check-in site key before forcing check-in captcha")
				return
			}
			if common.CaptchaType == "hcaptcha" && (!common.HCaptchaEnabled || common.HCaptchaSiteKey == "" || common.HCaptchaSecretKey == "") {
				common.ApiErrorMsg(c, "Configure and enable hCaptcha before forcing check-in captcha")
				return
			}
			if common.CaptchaType == "turnstile" && !common.TurnstileCheckEnabled {
				common.ApiErrorMsg(c, "Enable Turnstile before forcing check-in captcha")
				return
			}
		}
	case "ForceRedemptionCaptcha":
		if option.Value == "true" {
			if common.CaptchaType == "cap" && (!common.CapEnabled || common.CapSiteKey == "" || common.CapSecretKey == "") {
				common.ApiErrorMsg(c, "Configure and enable the Cap login site key before forcing redemption captcha")
				return
			}
			if common.CaptchaType == "hcaptcha" && (!common.HCaptchaEnabled || common.HCaptchaSiteKey == "" || common.HCaptchaSecretKey == "") {
				common.ApiErrorMsg(c, "Configure and enable hCaptcha before forcing redemption captcha")
				return
			}
			if common.CaptchaType == "turnstile" && (!common.TurnstileCheckEnabled || common.TurnstileSiteKey == "" || common.TurnstileSecretKey == "") {
				common.ApiErrorMsg(c, "Configure and enable Turnstile before forcing redemption captcha")
				return
			}
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "theme.frontend":
		if option.Value != "default" && option.Value != "classic" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的主题值，可选值：default（新版前端）、classic（经典前端）",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "GroupDefaultModel":
		err = ratio_setting.CheckGroupModelMap(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "GroupRetryTimes":
		err = ratio_setting.CheckGroupRetryTimes(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if err = syncCapDifficultyForOption(c, option.Key, option.Value.(string)); err != nil {
		common.ApiError(c, err)
		return
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 出于安全考虑只记录被修改的配置项名称，不记录配置值（可能含密钥等敏感信息）。
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": option.Key,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
