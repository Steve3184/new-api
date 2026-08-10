package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveWaffoPancakeSubscriptionTradeNoAcceptsOrderUserEmail(t *testing.T) {
	previousDB := model.DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, previousLogType)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionOrder{}))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	user := &model.User{
		Username: "waffo-email-user",
		Password: "password",
		Email:    "buyer@example.com",
	}
	require.NoError(t, db.Create(user).Error)
	order := &model.SubscriptionOrder{
		UserId:          user.Id,
		TradeNo:         "WAFFO_PANCAKE_SUB-email-identity",
		PaymentProvider: model.PaymentProviderWaffoPancake,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(order).Error)

	event := &WaffoPancakeWebhookEvent{Data: WaffoPancakeWebhookData{
		OrderMerchantExternalID:       order.TradeNo,
		MerchantProvidedBuyerIdentity: " Buyer@Example.com ",
	}}
	tradeNo, err := ResolveWaffoPancakeSubscriptionTradeNo(event)
	require.NoError(t, err)
	require.Equal(t, order.TradeNo, tradeNo)

	event.Data.MerchantProvidedBuyerIdentity = "other@example.com"
	_, err = ResolveWaffoPancakeSubscriptionTradeNo(event)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "buyer identity mismatch"))
}
