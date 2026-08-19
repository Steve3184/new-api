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

func TestCompressSupportImageScalesToTwoKAndUsesJpegQuality(t *testing.T) {
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
	decoded, format, err := image.Decode(bytes.NewReader(compressed))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Positive(t, decoded.Bounds().Dx())
	assert.Positive(t, decoded.Bounds().Dy())
	assert.LessOrEqual(t, decoded.Bounds().Dx(), SupportImageMaxDimension)
	assert.LessOrEqual(t, decoded.Bounds().Dy(), SupportImageMaxDimension)

	large := image.NewRGBA(image.Rect(0, 0, 4096, 2048))
	var largeSource bytes.Buffer
	require.NoError(t, png.Encode(&largeSource, large))
	resized, err := CompressSupportImage(largeSource.Bytes())
	require.NoError(t, err)
	largeDecoded, _, err := image.Decode(bytes.NewReader(resized))
	require.NoError(t, err)
	assert.Equal(t, SupportImageMaxDimension, largeDecoded.Bounds().Dx())
	assert.Equal(t, SupportImageMaxDimension/2, largeDecoded.Bounds().Dy())
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

func TestClearSupportDataDeletesMessagesAndConversations(t *testing.T) {
	truncateTables(t)
	conversation, err := GetOrCreateSupportConversation(7021)
	require.NoError(t, err)
	_, err = CreateSupportMessage(SupportMessageInput{
		ConversationId: conversation.Id,
		SenderId:       7021,
		SenderRole:     common.RoleCommonUser,
		Kind:           SupportMessageText,
		Content:        "delete me",
	})
	require.NoError(t, err)

	result, err := ClearSupportData()
	require.NoError(t, err)
	assert.EqualValues(t, 1, result.Conversations)
	assert.EqualValues(t, 1, result.Messages)

	var conversationCount int64
	var messageCount int64
	require.NoError(t, DB.Model(&SupportConversation{}).Count(&conversationCount).Error)
	require.NoError(t, DB.Model(&SupportMessage{}).Count(&messageCount).Error)
	assert.Zero(t, conversationCount)
	assert.Zero(t, messageCount)
}

func TestSupportMessageStoresAttachmentBytesOutsideTextColumn(t *testing.T) {
	truncateTables(t)
	conversation, err := GetOrCreateSupportConversation(7051)
	require.NoError(t, err)

	imageBytes := []byte{0xff, 0xd8, 0xff, 0xd9}
	message, err := CreateSupportMessage(SupportMessageInput{
		ConversationId: conversation.Id,
		SenderId:       7051,
		SenderRole:     common.RoleCommonUser,
		Kind:           SupportMessageImage,
		ImageBytes:     imageBytes,
		ImageMime:      "image/jpeg",
	})
	require.NoError(t, err)
	assert.Empty(t, message.ImageData)

	var stored SupportMessage
	require.NoError(t, DB.First(&stored, message.Id).Error)
	assert.Equal(t, imageBytes, stored.ImageDataBlob)
	assert.Empty(t, stored.ImageData)
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

func TestListSupportConversationsSearchesNumericAndUserText(t *testing.T) {
	truncateTables(t)
	users := []*User{
		{Id: 7401, Username: "numeric-search-user", DisplayName: "Numeric Search", AffCode: "support-search-7401", Password: "unused-password-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 7402, Username: "text-search-user", DisplayName: "Text Search", AffCode: "support-search-7402", Password: "unused-password-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	for _, user := range users {
		require.NoError(t, DB.Create(user).Error)
	}
	conversations := []*SupportConversation{
		{Id: 7411, UserId: users[0].Id, Title: "Numeric conversation"},
		{Id: 7412, UserId: users[1].Id, Title: "Text conversation"},
	}
	for _, conversation := range conversations {
		require.NoError(t, DB.Create(conversation).Error)
	}
	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}

	items, total, err := ListSupportConversations(pageInfo, "7411")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, 7411, items[0].Id)

	items, total, err = ListSupportConversations(pageInfo, "text-search-user")
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, 7412, items[0].Id)
}

func TestListSupportOrderQuotesIncludesSuccessfulOrdersAndSortsByTime(t *testing.T) {
	truncateTables(t)
	userId := 7501
	recentTime := int64(200)
	oldTime := int64(100)
	require.NoError(t, DB.Create(&TopUp{
		Id: 7511, UserId: userId, Amount: 25, Money: 12.5, TradeNo: "support-success-order",
		Status: common.TopUpStatusSuccess, PaymentProvider: PaymentProviderEpay, CreateTime: recentTime,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		Id: 7512, UserId: userId, Amount: 10, Money: 5, TradeNo: "support-pending-order",
		Status: common.TopUpStatusPending, PaymentProvider: PaymentProviderEpay, CreateTime: oldTime,
	}).Error)

	quotes, err := ListSupportOrderQuotes(userId)
	require.NoError(t, err)
	require.Len(t, quotes, 2)
	assert.Equal(t, common.TopUpStatusSuccess, quotes[0].Status)
	assert.Equal(t, int64(200), quotes[0].CreatedAt)
	assert.InDelta(t, 12.5, quotes[0].Money, 0.000001)
}

func TestGetSupportUnreadCountDoesNotCreateConversation(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&SupportConversation{Id: 7611, UserId: 7601, UnreadUser: 3, UnreadAdmin: 4}).Error)
	require.NoError(t, DB.Create(&SupportConversation{Id: 7612, UserId: 7602, UnreadUser: 8, UnreadAdmin: 2}).Error)

	userCount, err := GetSupportUnreadCount(7601, common.RoleCommonUser)
	require.NoError(t, err)
	assert.EqualValues(t, 3, userCount)

	adminCount, err := GetSupportUnreadCount(7601, common.RoleAdminUser)
	require.NoError(t, err)
	assert.EqualValues(t, 6, adminCount)

	var conversations int64
	require.NoError(t, DB.Model(&SupportConversation{}).Count(&conversations).Error)
	assert.EqualValues(t, 2, conversations)
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
