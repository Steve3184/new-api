package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Every instance may run this worker: the ledger status and wallet credit are
// committed together. Restarting also picks up overdue pending rewards.
func StartReferralRewardTask() {
	go func() {
		for {
			lastID := 0
			now := common.GetTimestamp()
			for {
				var rewards []model.ReferralReward
				err := model.DB.Where("status = ? AND available_at <= ? AND id > ?", "pending", now, lastID).Order("id").Limit(100).Find(&rewards).Error
				if err != nil {
					common.SysError("referral worker: " + err.Error())
					break
				}
				for _, reward := range rewards {
					if err := model.CreditReferralReward(reward.ID, now); err != nil {
						common.SysError("referral credit: " + err.Error())
					}
					lastID = reward.ID
				}
				if len(rewards) < 100 {
					break
				}
			}
			time.Sleep(time.Minute)
		}
	}()
}
