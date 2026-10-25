package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OAuthBindReward records every external account ever linked per provider,
// whether or not a reward was paid, so a binding reward is paid at most once
// per external account and at most once per user and provider, even after
// unbinding, account deletion or rebinding.
type OAuthBindReward struct {
	Id        int64  `json:"id" gorm:"primaryKey"`
	Provider  string `json:"provider" gorm:"type:varchar(64);not null;uniqueIndex:idx_oauth_bind_reward_subject,priority:1;uniqueIndex:idx_oauth_bind_reward_user,priority:1"`
	Subject   string `json:"subject" gorm:"type:varchar(256);not null;uniqueIndex:idx_oauth_bind_reward_subject,priority:2"`
	UserId    int    `json:"user_id" gorm:"not null;index;uniqueIndex:idx_oauth_bind_reward_user,priority:2"`
	Quota     int    `json:"quota" gorm:"not null;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (OAuthBindReward) TableName() string {
	return "oauth_bind_rewards"
}

// ClaimOAuthBindRewardWithTx records the first link of an external account
// and pays the configured reward when neither the account nor the user has
// claimed this provider before. Call it in the transaction that writes the
// binding and pass the result to FinishOAuthBindReward after commit. A wallet
// at its limit keeps the binding and is paid nothing.
func ClaimOAuthBindRewardWithTx(tx *gorm.DB, provider, subject string, userId int) (int, error) {
	if provider == "" || subject == "" || userId == 0 {
		return 0, nil
	}
	quota := operation_setting.GetOAuthBindReward(provider)
	claim := OAuthBindReward{Provider: provider, Subject: subject, UserId: userId, Quota: quota}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if result.Error != nil || result.RowsAffected == 0 || quota == 0 {
		return 0, result.Error
	}
	credit := tx.Model(&User{}).
		Where("id = ? AND quota <= ?", userId, common.MaxWalletQuota-quota).
		Update("quota", gorm.Expr("quota + ?", quota))
	if credit.Error != nil {
		return 0, credit.Error
	}
	if credit.RowsAffected == 1 {
		return quota, nil
	}
	return 0, tx.Model(&claim).Update("quota", 0).Error
}

// RecordOAuthBindClaimWithTx marks an external account as linked without
// paying a reward, for bindings an administrator sets or a migration rewrites.
func RecordOAuthBindClaimWithTx(tx *gorm.DB, provider, subject string, userId int) error {
	if provider == "" || subject == "" || userId == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&OAuthBindReward{Provider: provider, Subject: subject, UserId: userId}).Error
}

// FinishOAuthBindReward updates the quota cache and records the reward log
// once the binding transaction has committed.
func FinishOAuthBindReward(userId int, quota int, providerName string) {
	if quota <= 0 {
		return
	}
	gopool.Go(func() {
		if err := cacheIncrUserQuota(userId, int64(quota)); err != nil {
			common.SysLog(common.LogText("failed to increase user quota cache: %s", err.Error()))
		}
	})
	RecordLog(userId, LogTypeSystem, common.NewMessage("OAuth binding bonus for {{provider}} {{quota}}", map[string]any{"provider": providerName, "quota": logger.FormatQuota(quota)}))
}

// GetClaimedOAuthBindRewards returns the provider keys a user has claimed.
func GetClaimedOAuthBindRewards(userId int) ([]string, error) {
	providers := []string{}
	err := DB.Model(&OAuthBindReward{}).Where("user_id = ?", userId).Pluck("provider", &providers).Error
	return providers, err
}

// backfillOAuthBindRewards marks the bindings that existed before rewards
// were introduced as claimed, the first time the table is empty, so they can
// never be unbound and linked again for a reward.
func backfillOAuthBindRewards(db *gorm.DB) error {
	var existing int64
	if err := db.Model(&OAuthBindReward{}).Limit(1).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var users []User
		err := tx.Unscoped().
			Select("id", "github_id", "discord_id", "oidc_id", "linux_do_id", "telegram_id", "wechat_id").
			Where("github_id <> ? OR discord_id <> ? OR oidc_id <> ? OR linux_do_id <> ? OR telegram_id <> ? OR wechat_id <> ?", "", "", "", "", "", "").
			FindInBatches(&users, 300, func(_ *gorm.DB, _ int) error {
				rows := make([]OAuthBindReward, 0, len(users))
				for _, user := range users {
					subjects := map[string]string{
						"github":   user.GitHubId,
						"discord":  user.DiscordId,
						"oidc":     user.OidcId,
						"linuxdo":  user.LinuxDOId,
						"telegram": user.TelegramId,
						"wechat":   user.WeChatId,
					}
					for provider, subject := range subjects {
						if subject != "" {
							rows = append(rows, OAuthBindReward{Provider: provider, Subject: subject, UserId: user.Id})
						}
					}
				}
				return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
			}).Error
		if err != nil {
			return err
		}
		var bindings []UserOAuthBinding
		return tx.FindInBatches(&bindings, 300, func(_ *gorm.DB, _ int) error {
			rows := make([]OAuthBindReward, 0, len(bindings))
			for _, binding := range bindings {
				rows = append(rows, OAuthBindReward{Provider: operation_setting.OAuthBindRewardCustomKey(binding.ProviderId), Subject: binding.ProviderUserId, UserId: binding.UserId})
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
		}).Error
	})
}
