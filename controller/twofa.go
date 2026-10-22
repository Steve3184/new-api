package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Verify2FARequest struct {
	Code      string `json:"code" binding:"required"`
	FlowToken string `json:"flow_token,omitempty"`
}

type UpdateLoginTwoFARequest struct {
	Enabled *bool `json:"enabled"`
}

func Setup2FA(c *gin.Context) {
	authorization := middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopeTwoFASetup})
	if authorization == nil {
		return
	}
	identity, _ := middleware.GetStepUpIdentity(c)
	setup, err := service.StartTwoFASetup(identity, authorization)
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	recordUserSecurityAudit(c, identity.UserID, "user.2fa_setup", nil)
	common.ApiSuccess(c, setup)
}

func Enable2FA(c *gin.Context) {
	identity, ok := middleware.GetStepUpIdentity(c)
	if !ok {
		writeSecurityOperationError(c, service.ErrAuthTokenInvalid)
		return
	}
	var req Verify2FARequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := service.FinishTwoFASetup(identity, req.FlowToken, req.Code); err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	notificationFailed := service.NotifyUserSecurityChange(c, identity.UserID, i18n.MsgSecurityEventTwoFAEnabled) != nil
	recordUserSecurityAudit(c, identity.UserID, "user.2fa_enable", map[string]any{"notification_failed": notificationFailed})
	bundle, err := service.AdvanceCurrentSessionToUserVersion(identity, "twofa_enabled")
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	data := authRotationData(bundle)
	if notificationFailed {
		data["notification_warning"] = true
	}
	common.ApiSuccess(c, data)
}

// Disable2FA 禁用2FA
func Disable2FA(c *gin.Context) {
	if middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopeTwoFADisable}) == nil {
		return
	}
	identity, _ := middleware.GetStepUpIdentity(c)
	userId := identity.UserID
	if err := model.DisableTwoFAForSession(identity); err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	notificationFailed := service.NotifyUserSecurityChange(c, userId, i18n.MsgSecurityEventTwoFADisabled) != nil
	recordUserSecurityAudit(c, userId, "user.2fa_disable_self", map[string]any{"notification_failed": notificationFailed})
	bundle, err := service.AdvanceCurrentSessionToUserVersion(identity, "twofa_disabled")
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	data := authRotationData(bundle)
	if notificationFailed {
		data["notification_warning"] = true
	}
	common.ApiSuccessT(c, "Two-factor authentication disabled", data)
}

// Get2FAStatus 获取用户2FA状态
func Get2FAStatus(c *gin.Context) {
	userId := c.GetInt("id")

	twoFA, err := model.GetTwoFAByUserId(userId)
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	status := map[string]any{
		"enabled": false,
		"locked":  false,
	}

	if twoFA != nil {
		status["enabled"] = twoFA.IsEnabled
		status["locked"] = twoFA.IsLocked()
		if twoFA.IsEnabled {
			// 获取剩余备用码数量
			backupCount, err := model.GetUnusedBackupCodeCount(userId)
			if err != nil {
				common.SysLog(common.LogText("failed to count backup codes: %s", err.Error()))
			} else {
				status["backup_codes_remaining"] = backupCount
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}

func UpdateLoginTwoFA(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeSecurityOperationError(c, service.ErrAuthTokenInvalid)
		return
	}
	var req UpdateLoginTwoFARequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Enabled == nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if !*req.Enabled && middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopeLoginTwoFADisable}) == nil {
		return
	}
	if err := model.UpdateLoginTwoFactorSettingForSession(identity, *req.Enabled); err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	data := gin.H{"enabled": *req.Enabled}
	action := "user.login_2fa_enable"
	event := i18n.MsgSecurityEventLoginTwoFAEnabled
	if !*req.Enabled {
		action, event = "user.login_2fa_disable", i18n.MsgSecurityEventLoginTwoFADisabled
	}
	notificationFailed := service.NotifyUserSecurityChange(c, identity.UserID, event) != nil
	if notificationFailed {
		data["notification_warning"] = true
	}
	recordUserSecurityAudit(c, identity.UserID, action, map[string]any{"notification_failed": notificationFailed})
	if !*req.Enabled {
		bundle, err := service.AdvanceCurrentSessionToUserVersion(identity, "login_twofa_disabled")
		if err != nil {
			writeSecurityOperationError(c, err)
			return
		}
		for key, value := range authRotationData(bundle) {
			data[key] = value
		}
	}
	setAuthNoStore(c)
	common.ApiSuccess(c, data)
}

// RegenerateBackupCodes 重新生成备用码
func RegenerateBackupCodes(c *gin.Context) {
	if middleware.RequireSecurityProof(c, service.VerificationOperation{Scope: service.VerificationScopeTwoFABackupCodes}) == nil {
		return
	}
	identity, _ := middleware.GetStepUpIdentity(c)
	userId := identity.UserID
	// 生成新的备用码
	backupCodes, err := common.GenerateBackupCodes()
	if err != nil {
		common.ApiErrorT(c, "Failed to regenerate backup codes")
		common.SysLog(common.LogText("failed to generate backup codes: %s", err.Error()))
		return
	}

	// 保存新的备用码并原子推进用户鉴权版本
	if err := model.ReplaceBackupCodesForSession(identity, backupCodes); err != nil {
		common.ApiErrorT(c, "Failed to save backup codes")
		common.SysLog(common.LogText("failed to save backup codes: %s", err.Error()))
		return
	}
	notificationFailed := service.NotifyUserSecurityChange(c, userId, i18n.MsgSecurityEventBackupCodesRegenerated) != nil
	recordUserSecurityAudit(c, userId, "user.2fa_backup_codes", map[string]any{"notification_failed": notificationFailed})
	bundle, err := service.AdvanceCurrentSessionToUserVersion(identity, "twofa_backup_codes_regenerated")
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	data := authRotationData(bundle)
	if notificationFailed {
		data["notification_warning"] = true
	}
	data["backup_codes"] = backupCodes
	common.ApiSuccessT(c, "Backup codes regenerated successfully", data)
}

// Verify2FALogin 登录时验证2FA
func Verify2FALogin(c *gin.Context) {
	VerifyLogin(c)
}

// Admin2FAStats 管理员获取2FA统计信息
func Admin2FAStats(c *gin.Context) {
	stats, err := model.GetTwoFAStats()
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// AdminDisable2FA 管理员强制禁用用户2FA
func AdminDisable2FA(c *gin.Context) {
	userIdStr := c.Param("id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil || userId <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return
	}

	// 检查目标用户权限
	targetUser, err := model.GetUserById(userId, false)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorT(c, "User does not exist")
		return
	}
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	myRole := c.GetInt("role")
	if !canManageTargetRole(myRole, targetUser.Role) {
		common.ApiErrorT(c, "No permission to change the 2FA settings of users at the same or higher level")
		return
	}
	authorization := requireAdminUserProof(c, service.VerificationScopeAdminUserTwoFADisable, service.AdminUserContext{UserID: userId})
	if authorization == nil {
		return
	}

	// 禁用2FA
	if err := model.DisableTwoFAWithAuthVersion(userId); err != nil {
		if errors.Is(err, model.ErrTwoFANotEnabled) {
			common.ApiErrorT(c, "Two-factor authentication is not enabled.")
			return
		}
		writeSecurityOperationError(c, err)
		return
	}
	notificationFailed := service.NotifyAccountSecurityChange(service.AccountMailLanguage(c, userId), targetUser.Email, i18n.MsgSecurityEventTwoFAReset) != nil
	recordManageAuditFor(c, userId, "user.2fa_disable", map[string]any{
		"username": targetUser.Username, "verification_method": authorization.Method,
		"notification_failed": notificationFailed,
	})
	if _, err := model.RevokeAllUserSessions(userId, "admin_twofa_disabled"); err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	data := gin.H{}
	if notificationFailed {
		data["notification_warning"] = true
	}
	common.ApiSuccessT(c, "Two-factor authentication reset", data)
}
