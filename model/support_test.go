package model

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressSupportImageProducesSmallJpeg(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			canvas.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255})
		}
	}
	var source bytes.Buffer
	require.NoError(t, png.Encode(&source, canvas))

	compressed, err := CompressSupportImage(source.Bytes())
	require.NoError(t, err)
	assert.LessOrEqual(t, len(compressed), SupportImageMaxBytes)
	decoded, format, err := image.Decode(bytes.NewReader(compressed))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Positive(t, decoded.Bounds().Dx())
	assert.Positive(t, decoded.Bounds().Dy())
}

func TestSupportMessageRetentionKeepsConfiguredNewestMessages(t *testing.T) {
	truncateTables(t)
	previousLimit := common.SupportMessageLimit
	common.SupportMessageLimit = 3
	t.Cleanup(func() { common.SupportMessageLimit = previousLimit })

	conversation, err := GetOrCreateSupportConversation(7001)
	require.NoError(t, err)
	for index := 1; index <= 5; index++ {
		_, err = CreateSupportMessage(SupportMessageInput{
			ConversationId: conversation.Id,
			SenderId:       7001,
			SenderRole:     common.RoleCommonUser,
			Kind:           SupportMessageText,
			Content:        string(rune('0' + index)),
		})
		require.NoError(t, err)
	}

	var count int64
	require.NoError(t, DB.Model(&SupportMessage{}).Where("conversation_id = ?", conversation.Id).Count(&count).Error)
	assert.EqualValues(t, 3, count)
	messages, err := GetSupportMessages(conversation.Id, 10, 0)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, "3", messages[0].Content)
	assert.Equal(t, "5", messages[2].Content)
}

func TestSupportOrderQuoteRequiresOwnership(t *testing.T) {
	truncateTables(t)
	order := &TopUp{UserId: 7101, Amount: 10, Money: 10, TradeNo: "support-order-7101", Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderEpay}
	require.NoError(t, DB.Create(order).Error)

	quote, err := GetSupportOrderQuote(7101, SupportOrderTopUp, order.Id)
	require.NoError(t, err)
	assert.True(t, quote.CanComplete)
	assert.Equal(t, order.TradeNo, quote.TradeNo)
	_, err = GetSupportOrderQuote(7102, SupportOrderTopUp, order.Id)
	assert.Error(t, err)
}

func TestGrantSupportUserQuotaChecksWalletCeiling(t *testing.T) {
	truncateTables(t)
	user := &User{Id: 7201, Username: "support-grant-user", Password: "unused-password-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 100}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, GrantSupportUserQuota(user.Id, 50))
	var updated User
	require.NoError(t, DB.Select("quota").First(&updated, user.Id).Error)
	assert.Equal(t, 150, updated.Quota)
	assert.Error(t, GrantSupportUserQuota(user.Id, common.MaxQuota))
}

func TestGrantSupportUserQuotaWithMessageCommitsAsOneOperation(t *testing.T) {
	truncateTables(t)
	user := &User{Id: 7301, Username: "support-atomic-user", Password: "unused-password-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 100}
	require.NoError(t, DB.Create(user).Error)
	conversation, err := GetOrCreateSupportConversation(user.Id)
	require.NoError(t, err)

	message, err := GrantSupportUserQuotaWithMessage(
		conversation.Id,
		9001,
		common.RoleAdminUser,
		50,
		"manual support grant",
	)
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, SupportMessageQuotaGrant, message.Kind)

	var updated User
	require.NoError(t, DB.Select("quota").First(&updated, user.Id).Error)
	assert.Equal(t, 150, updated.Quota)
	var count int64
	require.NoError(t, DB.Model(&SupportMessage{}).Where("conversation_id = ?", conversation.Id).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
