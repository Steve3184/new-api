package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSaveWaffoPancakeConfigKeepsMaskedPrivateKey(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousOptionMap := common.OptionMap
	previousMerchantID := setting.WaffoPancakeMerchantID
	previousPrivateKey := setting.WaffoPancakePrivateKey
	previousReturnURL := setting.WaffoPancakeReturnURL
	previousStoreID := setting.WaffoPancakeStoreID
	previousProductID := setting.WaffoPancakeProductID

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	require.NoError(t, db.Create(&model.Option{
		Key:   "WaffoPancakePrivateKey",
		Value: "persisted-private-key",
	}).Error)

	model.DB, model.LOG_DB = db, db
	common.OptionMap = map[string]string{
		"WaffoPancakePrivateKey": "persisted-private-key",
	}
	setting.WaffoPancakePrivateKey = "persisted-private-key"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.OptionMap = previousOptionMap
		setting.WaffoPancakeMerchantID = previousMerchantID
		setting.WaffoPancakePrivateKey = previousPrivateKey
		setting.WaffoPancakeReturnURL = previousReturnURL
		setting.WaffoPancakeStoreID = previousStoreID
		setting.WaffoPancakeProductID = previousProductID
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, SaveWaffoPancakeConfig(
		context.Background(),
		"merchant",
		common.SensitiveOptionPlaceholder,
		"https://example.com/wallet",
		"store",
		"product",
	))

	var saved model.Option
	require.NoError(t, db.Where("key = ?", "WaffoPancakePrivateKey").First(&saved).Error)
	assert.Equal(t, "persisted-private-key", saved.Value)
	assert.Equal(t, "persisted-private-key", setting.WaffoPancakePrivateKey)
	assert.Equal(t, "persisted-private-key", common.OptionMap["WaffoPancakePrivateKey"])
}
