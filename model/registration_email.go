package model

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// RegistrationEmail keeps every email an account was registered with, even
// after the account changes or drops it, so a whitelisted address cannot earn
// registration rewards twice by being moved away and registered again.
type RegistrationEmail struct {
	Id        int    `json:"id"`
	Email     string `json:"email" gorm:"index"`
	UserId    int    `json:"user_id" gorm:"index"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func recordRegistrationEmailWithTx(tx *gorm.DB, email string, userId int) error {
	email = NormalizeEmail(email)
	if email == "" {
		return nil
	}
	return tx.Create(&RegistrationEmail{Email: email, UserId: userId}).Error
}

// registrationRewardEligibleWithTx reports whether an account registering
// with email may receive the sign-up, invitee and inviter bonuses. With the
// email domain reward restriction on, only a whitelisted address that has
// never been registered before qualifies. The inviter link is stored either
// way, so top-up referral rebates are unaffected.
func registrationRewardEligibleWithTx(tx *gorm.DB, email string) (bool, error) {
	if !common.EmailDomainRewardRestrictionEnabled {
		return true, nil
	}
	email = NormalizeEmail(email)
	_, domain, ok := strings.Cut(email, "@")
	if !ok || !slices.Contains(common.EmailDomainWhitelist, domain) {
		return false, nil
	}
	var count int64
	if err := tx.Model(&RegistrationEmail{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// backfillRegistrationEmails seeds the history from existing accounts the
// first time the table is empty, so addresses registered before this table
// existed are covered once their accounts move to another email.
func backfillRegistrationEmails(db *gorm.DB) error {
	var existing int64
	if err := db.Model(&RegistrationEmail{}).Limit(1).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var users []User
		return tx.Unscoped().Select("id", "email").Where("email <> ?", "").FindInBatches(&users, 300, func(batch *gorm.DB, _ int) error {
			rows := make([]RegistrationEmail, 0, len(users))
			for _, user := range users {
				rows = append(rows, RegistrationEmail{Email: NormalizeEmail(user.Email), UserId: user.Id})
			}
			return tx.Create(&rows).Error
		}).Error
	})
}
