package model

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"

	xdraw "golang.org/x/image/draw"
	"gorm.io/gorm"
)

const (
	SupportConversationOpen   = "open"
	SupportConversationClosed = "closed"

	SupportMessageText              = "text"
	SupportMessageImage             = "image"
	SupportMessageOrderQuote        = "order_quote"
	SupportMessageQuotaGrant        = "quota_grant"
	SupportMessageSubscriptionGrant = "subscription_grant"

	SupportOrderTopUp        = "topup"
	SupportOrderSubscription = "subscription"

	SupportImageMaxDimension  = 2048
	SupportImageMaxInputBytes = 8 * 1024 * 1024
	SupportTextMaxLength      = 4000
)

type SupportConversation struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"uniqueIndex"`
	Title         string `json:"title" gorm:"type:varchar(128)"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	LastMessageAt int64  `json:"last_message_at" gorm:"index"`
	UnreadUser    int    `json:"unread_user"`
	UnreadAdmin   int    `json:"unread_admin"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	Username      string `json:"username" gorm:"-"`
	DisplayName   string `json:"display_name" gorm:"-"`
}

type SupportMessage struct {
	Id             int    `json:"id"`
	ConversationId int    `json:"conversation_id" gorm:"index"`
	SenderId       int    `json:"sender_id" gorm:"index"`
	SenderRole     int    `json:"sender_role"`
	Kind           string `json:"kind" gorm:"type:varchar(32);index"`
	Content        string `json:"content" gorm:"type:text"`
	ImageData      string `json:"image_data,omitempty" gorm:"type:text"`
	// ImageDataBlob stores new attachments as binary data. ImageData is kept for
	// backwards compatibility with messages written before the blob column.
	ImageDataBlob  []byte  `json:"-" gorm:"column:image_data_blob"`
	ImageMime      string  `json:"image_mime,omitempty" gorm:"type:varchar(64)"`
	ImageSize      int     `json:"image_size,omitempty"`
	OrderType      string  `json:"order_type,omitempty" gorm:"type:varchar(32)"`
	OrderId        int     `json:"order_id,omitempty"`
	OrderTradeNo   string  `json:"order_trade_no,omitempty" gorm:"type:varchar(255)"`
	OrderStatus    string  `json:"order_status,omitempty" gorm:"type:varchar(32)"`
	OrderProvider  string  `json:"order_provider,omitempty" gorm:"type:varchar(64)"`
	OrderAmount    int64   `json:"order_amount,omitempty"`
	OrderMoney     float64 `json:"order_money,omitempty"`
	OrderPlanId    int     `json:"order_plan_id,omitempty"`
	OrderPlanTitle string  `json:"order_plan_title,omitempty" gorm:"type:varchar(255)"`
	GrantQuota     int     `json:"grant_quota,omitempty"`
	GrantPlanId    int     `json:"grant_plan_id,omitempty"`
	GrantPlanTitle string  `json:"grant_plan_title,omitempty" gorm:"type:varchar(255)"`
	CreatedAt      int64   `json:"created_at" gorm:"index"`
}

type SupportOrderQuote struct {
	OrderType   string  `json:"order_type"`
	OrderId     int     `json:"order_id"`
	TradeNo     string  `json:"trade_no"`
	Status      string  `json:"status"`
	Provider    string  `json:"provider"`
	Amount      int64   `json:"amount,omitempty"`
	Money       float64 `json:"money,omitempty"`
	PlanId      int     `json:"plan_id,omitempty"`
	PlanTitle   string  `json:"plan_title,omitempty"`
	CreatedAt   int64   `json:"created_at"`
	CanComplete bool    `json:"can_complete"`
}

type ClearedSupportData struct {
	Conversations int64 `json:"conversations"`
	Messages      int64 `json:"messages"`
}

func ClearSupportData() (ClearedSupportData, error) {
	result := ClearedSupportData{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		messageDelete := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SupportMessage{})
		if messageDelete.Error != nil {
			return messageDelete.Error
		}
		result.Messages = messageDelete.RowsAffected

		conversationDelete := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&SupportConversation{})
		if conversationDelete.Error != nil {
			return conversationDelete.Error
		}
		result.Conversations = conversationDelete.RowsAffected
		return nil
	})
	return result, err
}

func (message *SupportMessage) BeforeCreate(tx *gorm.DB) error {
	if message.CreatedAt == 0 {
		message.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func (conversation *SupportConversation) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if conversation.CreatedAt == 0 {
		conversation.CreatedAt = now
	}
	if conversation.UpdatedAt == 0 {
		conversation.UpdatedAt = now
	}
	if conversation.Title == "" {
		conversation.Title = "服务支持"
	}
	if conversation.Status == "" {
		conversation.Status = SupportConversationOpen
	}
	return nil
}

func GetOrCreateSupportConversation(userId int) (*SupportConversation, error) {
	if userId <= 0 {
		return nil, errors.New("无效的用户 ID")
	}
	var conversation SupportConversation
	if err := DB.Where("user_id = ?", userId).First(&conversation).Error; err == nil {
		return &conversation, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	conversation = SupportConversation{UserId: userId, Title: "服务支持", Status: SupportConversationOpen}
	if err := DB.Create(&conversation).Error; err != nil {
		// Another request may have created the unique user conversation between
		// the initial lookup and INSERT. Re-read after any INSERT failure so this
		// path also works with drivers that do not translate duplicate-key errors.
		if findErr := DB.Where("user_id = ?", userId).First(&conversation).Error; findErr == nil {
			return &conversation, nil
		}
		return nil, err
	}
	return &conversation, nil
}

func GetSupportConversationById(id int) (*SupportConversation, error) {
	if id <= 0 {
		return nil, errors.New("无效的会话 ID")
	}
	var conversation SupportConversation
	if err := DB.First(&conversation, id).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

func GetUserSupportConversation(userId int) (*SupportConversation, error) {
	return GetOrCreateSupportConversation(userId)
}

func GetSupportUnreadCount(userId int, userRole int) (int64, error) {
	query := DB.Model(&SupportConversation{})
	if userRole >= common.RoleAdminUser {
		query = query.Select("COALESCE(SUM(unread_admin), 0)")
	} else {
		if userId <= 0 {
			return 0, errors.New("无效的用户 ID")
		}
		query = query.Select("COALESCE(SUM(unread_user), 0)").Where("user_id = ?", userId)
	}
	var count int64
	if err := query.Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func ListSupportConversations(pageInfo *common.PageInfo, keyword string) ([]*SupportConversation, int64, error) {
	keyword = strings.TrimSpace(keyword)
	query := DB.Model(&SupportConversation{})
	if keyword != "" {
		pattern := "%" + keyword + "%"
		conditions := []string{"users.username LIKE ?", "users.display_name LIKE ?"}
		args := []any{pattern, pattern}
		if id, err := strconv.Atoi(keyword); err == nil {
			conditions = append([]string{"support_conversations.id = ?", "support_conversations.user_id = ?"}, conditions...)
			args = append([]any{id, id}, args...)
		}
		query = query.Joins("JOIN users ON users.id = support_conversations.user_id").Where(strings.Join(conditions, " OR "), args...)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var conversations []*SupportConversation
	listQuery := query.Order("support_conversations.last_message_at desc, support_conversations.id desc").
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx())
	if err := listQuery.Find(&conversations).Error; err != nil {
		return nil, 0, err
	}
	if len(conversations) == 0 {
		return conversations, total, nil
	}
	userIds := make([]int, 0, len(conversations))
	for _, conversation := range conversations {
		userIds = append(userIds, conversation.UserId)
	}
	var users []User
	if err := DB.Select("id, username, display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	usersById := make(map[int]User, len(users))
	for _, user := range users {
		usersById[user.Id] = user
	}
	for _, conversation := range conversations {
		if user, ok := usersById[conversation.UserId]; ok {
			conversation.Username = user.Username
			conversation.DisplayName = user.DisplayName
		}
	}
	return conversations, total, nil
}

func GetSupportMessages(conversationId int, limit int, beforeId int) ([]*SupportMessage, error) {
	if conversationId <= 0 {
		return nil, errors.New("无效的会话 ID")
	}
	configuredLimit := common.SupportMessageLimit
	if configuredLimit <= 0 {
		configuredLimit = 100
	}
	if limit <= 0 || limit > configuredLimit {
		limit = configuredLimit
	}
	query := DB.Where("conversation_id = ?", conversationId)
	if beforeId > 0 {
		query = query.Where("id < ?", beforeId)
	}
	var messages []*SupportMessage
	if err := query.Order("id desc").Limit(limit).Find(&messages).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func MarkSupportConversationRead(conversationId int, userRole int) error {
	field := "unread_user"
	if userRole >= common.RoleAdminUser {
		field = "unread_admin"
	}
	return DB.Model(&SupportConversation{}).Where("id = ?", conversationId).Update(field, 0).Error
}

type SupportMessageInput struct {
	ConversationId int
	SenderId       int
	SenderRole     int
	Kind           string
	Content        string
	ImageData      string
	ImageBytes     []byte
	ImageMime      string
	ImageSize      int
	OrderQuote     *SupportOrderQuote
	GrantQuota     int
	GrantPlanId    int
	GrantPlanTitle string
}

func (input SupportMessageInput) normalized() (*SupportMessage, error) {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Content = strings.TrimSpace(input.Content)
	if input.SenderId <= 0 || input.ConversationId <= 0 {
		return nil, errors.New("无效的消息归属")
	}
	if input.Kind == "" {
		if input.ImageData != "" || len(input.ImageBytes) > 0 {
			input.Kind = SupportMessageImage
		} else {
			input.Kind = SupportMessageText
		}
	}
	switch input.Kind {
	case SupportMessageText, SupportMessageImage, SupportMessageOrderQuote, SupportMessageQuotaGrant, SupportMessageSubscriptionGrant:
	default:
		return nil, errors.New("不支持的消息类型")
	}
	if len(input.Content) > SupportTextMaxLength {
		return nil, fmt.Errorf("消息不能超过 %d 个字符", SupportTextMaxLength)
	}
	if input.Kind == SupportMessageImage {
		if len(input.ImageBytes) > 0 {
			input.ImageSize = len(input.ImageBytes)
		}
		if input.ImageData == "" && len(input.ImageBytes) == 0 {
			return nil, errors.New("图片大小或内容无效")
		}
		if input.ImageSize <= 0 || input.ImageSize > SupportImageMaxInputBytes {
			return nil, errors.New("图片大小或内容无效")
		}
		input.ImageMime = "image/jpeg"
	}
	if input.Kind == SupportMessageOrderQuote {
		if input.OrderQuote == nil || input.OrderQuote.OrderId <= 0 || input.OrderQuote.TradeNo == "" {
			return nil, errors.New("订单引用无效")
		}
	}
	if input.Kind == SupportMessageQuotaGrant && input.GrantQuota <= 0 {
		return nil, errors.New("额度必须大于零")
	}
	if input.Kind == SupportMessageSubscriptionGrant && input.GrantPlanId <= 0 {
		return nil, errors.New("订阅套餐无效")
	}
	if input.SenderRole < common.RoleAdminUser && (input.Kind == SupportMessageQuotaGrant || input.Kind == SupportMessageSubscriptionGrant) {
		return nil, errors.New("该消息类型仅限管理员发送")
	}
	message := &SupportMessage{
		ConversationId: input.ConversationId,
		SenderId:       input.SenderId,
		SenderRole:     input.SenderRole,
		Kind:           input.Kind,
		Content:        input.Content,
		ImageData:      input.ImageData,
		ImageDataBlob:  append([]byte(nil), input.ImageBytes...),
		ImageMime:      input.ImageMime,
		ImageSize:      input.ImageSize,
		GrantQuota:     input.GrantQuota,
		GrantPlanId:    input.GrantPlanId,
		GrantPlanTitle: input.GrantPlanTitle,
	}
	if input.OrderQuote != nil {
		message.OrderType = input.OrderQuote.OrderType
		message.OrderId = input.OrderQuote.OrderId
		message.OrderTradeNo = input.OrderQuote.TradeNo
		message.OrderStatus = input.OrderQuote.Status
		message.OrderProvider = input.OrderQuote.Provider
		message.OrderAmount = input.OrderQuote.Amount
		message.OrderMoney = input.OrderQuote.Money
		message.OrderPlanId = input.OrderQuote.PlanId
		message.OrderPlanTitle = input.OrderQuote.PlanTitle
	}
	return message, nil
}

func CreateSupportMessage(input SupportMessageInput) (*SupportMessage, error) {
	message, err := input.normalized()
	if err != nil {
		return nil, err
	}
	message.CreatedAt = common.GetTimestamp()
	err = DB.Transaction(func(tx *gorm.DB) error {
		return createSupportMessageTx(tx, message)
	})
	if err != nil {
		return nil, err
	}
	return message, nil
}

func createSupportMessageTx(tx *gorm.DB, message *SupportMessage) error {
	var conversation SupportConversation
	if err := lockForUpdate(tx).Where("id = ?", message.ConversationId).First(&conversation).Error; err != nil {
		return err
	}
	return createSupportMessageTxLocked(tx, message, &conversation)
}

func createSupportMessageTxLocked(tx *gorm.DB, message *SupportMessage, conversation *SupportConversation) error {
	if err := tx.Create(message).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{
		"last_message_at": message.CreatedAt,
		"updated_at":      message.CreatedAt,
		"status":          SupportConversationOpen,
	}
	if message.SenderRole >= common.RoleAdminUser {
		updates["unread_user"] = gorm.Expr("unread_user + ?", 1)
	} else {
		updates["unread_admin"] = gorm.Expr("unread_admin + ?", 1)
	}
	if err := tx.Model(&SupportConversation{}).Where("id = ?", conversation.Id).Updates(updates).Error; err != nil {
		return err
	}
	limit := common.SupportMessageLimit
	if limit <= 0 {
		limit = 100
	}
	var oldIds []int
	if err := tx.Model(&SupportMessage{}).Where("conversation_id = ?", conversation.Id).Order("id desc").Offset(limit).Pluck("id", &oldIds).Error; err != nil {
		return err
	}
	if len(oldIds) > 0 {
		return tx.Where("id IN ?", oldIds).Delete(&SupportMessage{}).Error
	}
	return nil
}

func GetSupportOrderQuote(userId int, orderType string, orderId int) (*SupportOrderQuote, error) {
	if userId <= 0 || orderId <= 0 {
		return nil, errors.New("订单参数无效")
	}
	switch strings.ToLower(strings.TrimSpace(orderType)) {
	case SupportOrderTopUp:
		var order TopUp
		if err := DB.Where("id = ? AND user_id = ?", orderId, userId).First(&order).Error; err != nil {
			return nil, errors.New("充值订单不存在或不属于当前用户")
		}
		return &SupportOrderQuote{
			OrderType: SupportOrderTopUp, OrderId: order.Id, TradeNo: order.TradeNo,
			Status: order.Status, Provider: order.PaymentProvider, Amount: order.Amount, Money: order.Money,
			CreatedAt:   order.CreateTime,
			CanComplete: order.Status == common.TopUpStatusPending && order.PaymentProvider != PaymentProviderMonero,
		}, nil
	case SupportOrderSubscription:
		var order SubscriptionOrder
		if err := DB.Where("id = ? AND user_id = ?", orderId, userId).First(&order).Error; err != nil {
			return nil, errors.New("订阅订单不存在或不属于当前用户")
		}
		quote := &SupportOrderQuote{
			OrderType: SupportOrderSubscription, OrderId: order.Id, TradeNo: order.TradeNo,
			Status: order.Status, Provider: order.PaymentProvider, Money: order.Money, PlanId: order.PlanId,
			CreatedAt: order.CreateTime,
		}
		if plan, err := GetSubscriptionPlanById(order.PlanId); err == nil {
			quote.PlanTitle = plan.Title
		}
		return quote, nil
	default:
		return nil, errors.New("不支持的订单类型")
	}
}

func ListSupportOrderQuotes(userId int) ([]SupportOrderQuote, error) {
	if userId <= 0 {
		return nil, errors.New("无效的用户 ID")
	}
	var topups []TopUp
	if err := DB.Where("user_id = ?", userId).Order("id desc").Limit(20).Find(&topups).Error; err != nil {
		return nil, err
	}
	var subscriptions []SubscriptionOrder
	if err := DB.Where("user_id = ?", userId).Order("id desc").Limit(20).Find(&subscriptions).Error; err != nil {
		return nil, err
	}
	quotes := make([]SupportOrderQuote, 0, len(topups)+len(subscriptions))
	for _, order := range topups {
		quotes = append(quotes, SupportOrderQuote{OrderType: SupportOrderTopUp, OrderId: order.Id, TradeNo: order.TradeNo, Status: order.Status, Provider: order.PaymentProvider, Amount: order.Amount, Money: order.Money, CreatedAt: order.CreateTime, CanComplete: order.Status == common.TopUpStatusPending && order.PaymentProvider != PaymentProviderMonero})
	}
	for _, order := range subscriptions {
		quote := SupportOrderQuote{OrderType: SupportOrderSubscription, OrderId: order.Id, TradeNo: order.TradeNo, Status: order.Status, Provider: order.PaymentProvider, Money: order.Money, PlanId: order.PlanId, CreatedAt: order.CreateTime}
		if plan, err := GetSubscriptionPlanById(order.PlanId); err == nil {
			quote.PlanTitle = plan.Title
		}
		quotes = append(quotes, quote)
	}
	sort.SliceStable(quotes, func(i, j int) bool {
		if quotes[i].CreatedAt != quotes[j].CreatedAt {
			return quotes[i].CreatedAt > quotes[j].CreatedAt
		}
		return quotes[i].OrderId > quotes[j].OrderId
	})
	return quotes, nil
}

func RefreshSupportOrderQuote(messageId int) error {
	var message SupportMessage
	if err := DB.First(&message, messageId).Error; err != nil {
		return err
	}
	quote, err := GetSupportOrderQuoteForMessage(message)
	if err != nil {
		return err
	}
	return DB.Model(&SupportMessage{}).Where("id = ?", messageId).Updates(map[string]interface{}{
		"order_status":     quote.Status,
		"order_provider":   quote.Provider,
		"order_amount":     quote.Amount,
		"order_money":      quote.Money,
		"order_plan_id":    quote.PlanId,
		"order_plan_title": quote.PlanTitle,
	}).Error
}

func GetSupportOrderQuoteForMessage(message SupportMessage) (*SupportOrderQuote, error) {
	if message.OrderType == "" || message.OrderId <= 0 {
		return nil, errors.New("消息未引用订单")
	}
	var userId int
	var conversation SupportConversation
	if err := DB.Select("user_id").First(&conversation, message.ConversationId).Error; err != nil {
		return nil, err
	}
	userId = conversation.UserId
	return GetSupportOrderQuote(userId, message.OrderType, message.OrderId)
}

func GrantSupportUserQuota(userId int, quota int) error {
	if userId <= 0 || quota <= 0 || quota >= common.MaxQuota {
		return errors.New("额度范围无效")
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return grantSupportUserQuotaTx(tx, userId, quota)
	}); err != nil {
		return err
	}
	syncCreditUserQuotaCache(userId, quota, "support quota grant")
	return nil
}

func grantSupportUserQuotaTx(tx *gorm.DB, userId int, quota int) error {
	var user User
	if err := lockForUpdate(tx).Select("id, quota").First(&user, userId).Error; err != nil {
		return err
	}
	if user.Quota > common.MaxQuota-quota {
		return errors.New("用户额度将超过系统上限")
	}
	return tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", quota)).Error
}

// GrantSupportUserQuotaWithMessage applies the quota and records the audit
// message in one transaction, so a failed message insert cannot leave an
// untracked balance change.
func GrantSupportUserQuotaWithMessage(conversationId int, senderId int, senderRole int, quota int, note string) (*SupportMessage, error) {
	if conversationId <= 0 || senderId <= 0 || quota <= 0 || quota >= common.MaxQuota {
		return nil, errors.New("额度参数无效")
	}
	if strings.TrimSpace(note) == "" {
		note = fmt.Sprintf("管理员已直接发放 %d 额度", quota)
	}
	var message *SupportMessage
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var conversation SupportConversation
		if err := lockForUpdate(tx).Where("id = ?", conversationId).First(&conversation).Error; err != nil {
			return err
		}
		if err := grantSupportUserQuotaTx(tx, conversation.UserId, quota); err != nil {
			return err
		}
		userId = conversation.UserId
		createdMessage, err := (SupportMessageInput{
			ConversationId: conversationId,
			SenderId:       senderId,
			SenderRole:     senderRole,
			Kind:           SupportMessageQuotaGrant,
			Content:        note,
			GrantQuota:     quota,
		}).normalized()
		if err != nil {
			return err
		}
		message = createdMessage
		message.CreatedAt = common.GetTimestamp()
		return createSupportMessageTxLocked(tx, message, &conversation)
	})
	if err != nil {
		return nil, err
	}
	syncCreditUserQuotaCache(userId, quota, "support quota grant")
	return message, nil
}

// GrantSupportSubscriptionWithMessage grants a subscription and records the
// corresponding support message atomically.
func GrantSupportSubscriptionWithMessage(conversationId int, senderId int, senderRole int, planId int, note string) (*SupportMessage, error) {
	if conversationId <= 0 || senderId <= 0 || planId <= 0 {
		return nil, errors.New("订阅参数无效")
	}
	var message *SupportMessage
	var userId int
	var groupChanged bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var conversation SupportConversation
		if err := lockForUpdate(tx).Where("id = ?", conversationId).First(&conversation).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			return err
		}
		var userRow User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", conversation.UserId).First(&userRow).Error; err != nil {
			return err
		}
		subscription, err := CreateUserSubscriptionFromPlanTx(tx, conversation.UserId, plan, "support")
		if err != nil {
			return err
		}
		userId = conversation.UserId
		groupChanged = subscription.PrevUserGroup != ""
		if strings.TrimSpace(note) == "" {
			note = fmt.Sprintf("管理员已直接发放订阅：%s", plan.Title)
		}
		message, err = (SupportMessageInput{
			ConversationId: conversationId,
			SenderId:       senderId,
			SenderRole:     senderRole,
			Kind:           SupportMessageSubscriptionGrant,
			Content:        note,
			GrantPlanId:    planId,
			GrantPlanTitle: plan.Title,
		}).normalized()
		if err != nil {
			return err
		}
		message.CreatedAt = common.GetTimestamp()
		return createSupportMessageTxLocked(tx, message, &conversation)
	})
	if err != nil {
		return nil, err
	}
	InvalidateUserSubscriptionRateLimitCache(userId)
	if groupChanged {
		refreshSubscriptionUserGroupCache(userId, "support subscription grant")
	}
	return message, nil
}

// CompressSupportImage converts an uploaded image to JPEG quality 90 and
// scales it proportionally so neither dimension exceeds 2K.
func CompressSupportImage(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > SupportImageMaxInputBytes {
		return nil, errors.New("图片文件过大或为空")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法读取图片: %w", err)
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || int64(width)*int64(height) > 16_000_000 {
		return nil, errors.New("图片尺寸无效")
	}
	current := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(current, current.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(current, current.Bounds(), source, bounds.Min, draw.Over)
	maxDimension := max(width, height)
	if maxDimension > SupportImageMaxDimension {
		scale := float64(SupportImageMaxDimension) / float64(maxDimension)
		newWidth := max(1, int(float64(width)*scale+0.5))
		newHeight := max(1, int(float64(height)*scale+0.5))
		resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		xdraw.CatmullRom.Scale(resized, resized.Bounds(), current, current.Bounds(), xdraw.Over, nil)
		current = resized
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, current, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("图片压缩失败: %w", err)
	}
	return encoded.Bytes(), nil
}

func ReadLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("上传内容超过大小限制")
	}
	return data, nil
}
