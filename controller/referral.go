package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetReferralRewards(c *gin.Context)      { listReferralRewards(c, false) }
func AdminGetReferralRewards(c *gin.Context) { listReferralRewards(c, true) }

func listReferralRewards(c *gin.Context, admin bool) {
	page := common.GetPageQuery(c)
	query := model.DB.Model(&model.ReferralReward{})
	if !admin {
		query = query.Where("inviter_id = ?", c.GetInt("id"))
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if admin {
		for _, field := range []string{"inviter_id", "invitee_id"} {
			if value := c.Query(field); value != "" {
				id, err := strconv.Atoi(value)
				if err != nil || id <= 0 {
					common.ApiErrorMsg(c, "invalid user ID")
					return
				}
				query = query.Where(field+" = ?", id)
			}
		}
		if tradeNo := c.Query("trade_no"); tradeNo != "" {
			query = query.Where("trade_no = ?", tradeNo)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var records []model.ReferralReward
	if err := query.Order("id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&records).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	// Users receive only their own ledger, without payment identifiers or admin notes.
	if !admin {
		for i := range records {
			records[i].TradeNo = ""
			records[i].Reason = ""
			records[i].OperatorID = 0
			records[i].TopUpID = 0
		}
	}
	totals := []struct {
		Status string `json:"status"`
		Quota  int64  `json:"quota"`
		Count  int64  `json:"count"`
	}{}
	summary := model.DB.Model(&model.ReferralReward{})
	if !admin {
		summary = summary.Where("inviter_id = ?", c.GetInt("id"))
	}
	if err := summary.Select("status, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS count").Group("status").Scan(&totals).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	rules := operation_setting.GetReferralSetting()
	var invitedUsers int64
	if !admin {
		if err := model.DB.Model(&model.User{}).Where("inviter_id = ?", c.GetInt("id")).Count(&invitedUsers).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	}
	common.ApiSuccess(c, gin.H{"items": records, "total": total, "summary": totals, "rules": rules, "invited_users": invitedUsers})
}

func AdminReverseReferralReward(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid reward ID")
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ReverseReferralReward(id, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
