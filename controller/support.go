package controller

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type supportMessageRequest struct {
	Kind        string `json:"kind"`
	Content     string `json:"content"`
	OrderType   string `json:"order_type"`
	OrderId     int    `json:"order_id"`
	ImageData   string `json:"image_data"`
	GrantQuota  int    `json:"grant_quota"`
	GrantPlanId int    `json:"grant_plan_id"`
}

type supportConversationResponse struct {
	Conversation *model.SupportConversation `json:"conversation"`
	Messages     []*model.SupportMessage    `json:"messages"`
}

func parseSupportMessageRequest(c *gin.Context) (supportMessageRequest, []byte, error) {
	var request supportMessageRequest
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
		request.Kind = strings.TrimSpace(c.PostForm("kind"))
		request.Content = c.PostForm("content")
		request.OrderType = c.PostForm("order_type")
		request.OrderId, _ = strconv.Atoi(c.PostForm("order_id"))
		request.GrantQuota, _ = strconv.Atoi(c.PostForm("grant_quota"))
		request.GrantPlanId, _ = strconv.Atoi(c.PostForm("grant_plan_id"))
		file, err := c.FormFile("image")
		if err == nil {
			if file.Size <= 0 || file.Size > model.SupportImageMaxInputBytes {
				return request, nil, errors.New("图片文件过大")
			}
			handle, err := file.Open()
			if err != nil {
				return request, nil, err
			}
			defer handle.Close()
			data, err := model.ReadLimited(handle, model.SupportImageMaxInputBytes)
			if err != nil {
				return request, nil, err
			}
			return request, data, nil
		}
		if request.Kind == model.SupportMessageImage {
			return request, nil, errors.New("请选择图片")
		}
		return request, nil, nil
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		return request, nil, errors.New("请求参数无效")
	}
	return request, nil, nil
}

func buildSupportMessageInput(c *gin.Context, request supportMessageRequest, imageData []byte, conversationId int, senderId int, senderRole int) (model.SupportMessageInput, error) {
	input := model.SupportMessageInput{
		ConversationId: conversationId,
		SenderId:       senderId,
		SenderRole:     senderRole,
		Kind:           strings.TrimSpace(request.Kind),
		Content:        request.Content,
		GrantQuota:     request.GrantQuota,
		GrantPlanId:    request.GrantPlanId,
	}
	if len(imageData) > 0 || request.ImageData != "" {
		if strings.TrimSpace(request.Kind) == model.SupportMessageOrderQuote {
			return input, errors.New("订单引用不能同时附带图片")
		}
		if len(imageData) == 0 {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(request.ImageData))
			if err != nil {
				return input, errors.New("图片编码无效")
			}
			imageData = decoded
		}
		compressed, err := model.CompressSupportImage(imageData)
		if err != nil {
			return input, err
		}
		input.Kind = model.SupportMessageImage
		input.ImageBytes = compressed
		input.ImageMime = "image/jpeg"
		input.ImageSize = len(compressed)
	}
	if input.Kind == model.SupportMessageOrderQuote {
		quoteUserId := c.GetInt("id")
		if senderRole >= common.RoleAdminUser {
			conversation, err := model.GetSupportConversationById(conversationId)
			if err != nil {
				return input, err
			}
			quoteUserId = conversation.UserId
		}
		quote, err := model.GetSupportOrderQuote(quoteUserId, request.OrderType, request.OrderId)
		if err != nil {
			return input, err
		}
		input.OrderQuote = quote
	}
	return input, nil
}

func supportConversationPayload(conversation *model.SupportConversation, role int) (supportConversationResponse, error) {
	if conversation == nil {
		return supportConversationResponse{}, errors.New("会话不存在")
	}
	if err := model.MarkSupportConversationRead(conversation.Id, role); err != nil {
		return supportConversationResponse{}, err
	}
	messages, err := model.GetSupportMessages(conversation.Id, common.SupportMessageLimit, 0)
	if err != nil {
		return supportConversationResponse{}, err
	}
	for _, message := range messages {
		SupportImageResponse(message)
	}
	if role >= common.RoleAdminUser {
		conversation.UnreadAdmin = 0
	} else {
		conversation.UnreadUser = 0
	}
	return supportConversationResponse{Conversation: conversation, Messages: messages}, nil
}

func GetSupportConversation(c *gin.Context) {
	conversation, err := model.GetUserSupportConversation(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := supportConversationPayload(conversation, c.GetInt("role"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func GetSupportUnread(c *gin.Context) {
	count, err := model.GetSupportUnreadCount(c.GetInt("id"), c.GetInt("role"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"count": count})
}

func GetSupportOrders(c *gin.Context) {
	orders, err := model.ListSupportOrderQuotes(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func createSupportMessage(c *gin.Context, conversationId int, senderRole int) {
	request, imageData, err := parseSupportMessageRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	kind := strings.TrimSpace(request.Kind)
	if senderRole >= common.RoleAdminUser && (kind == model.SupportMessageQuotaGrant || kind == model.SupportMessageSubscriptionGrant) {
		common.ApiErrorMsg(c, "额度和订阅发放请使用专用操作")
		return
	}
	input, err := buildSupportMessageInput(c, request, imageData, conversationId, c.GetInt("id"), senderRole)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	message, err := model.CreateSupportMessage(input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	SupportImageResponse(message)
	common.ApiSuccess(c, message)
}

func SendSupportMessage(c *gin.Context) {
	conversation, err := model.GetUserSupportConversation(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	createSupportMessage(c, conversation.Id, c.GetInt("role"))
}

func AdminListSupportConversations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	conversations, total, err := model.ListSupportConversations(pageInfo, c.Query("keyword"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(conversations)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetSupportConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "会话 ID 无效")
		return
	}
	conversation, err := model.GetSupportConversationById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := supportConversationPayload(conversation, c.GetInt("role"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, payload)
}

func AdminSendSupportMessage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "会话 ID 无效")
		return
	}
	if _, err := model.GetSupportConversationById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	createSupportMessage(c, id, c.GetInt("role"))
}

type supportQuotaGrantRequest struct {
	Quota int    `json:"quota"`
	Note  string `json:"note"`
}

func AdminGrantSupportQuota(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "会话 ID 无效")
		return
	}
	var request supportQuotaGrantRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	conversation, err := model.GetSupportConversationById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	note := strings.TrimSpace(request.Note)
	if note == "" {
		note = fmt.Sprintf("管理员已直接发放 %d 额度", request.Quota)
	}
	message, err := model.GrantSupportUserQuotaWithMessage(id, c.GetInt("id"), c.GetInt("role"), request.Quota, note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(conversation.UserId, model.LogTypeManage, note, auditOperatorInfo(c))
	common.ApiSuccess(c, message)
}

type supportSubscriptionGrantRequest struct {
	PlanId int    `json:"plan_id"`
	Note   string `json:"note"`
}

func AdminGrantSupportSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "会话 ID 无效")
		return
	}
	var request supportSubscriptionGrantRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	conversation, err := model.GetSupportConversationById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	plan, err := model.GetSubscriptionPlanById(request.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	note := strings.TrimSpace(request.Note)
	if note == "" {
		note = fmt.Sprintf("管理员已直接发放订阅：%s", plan.Title)
	}
	message, err := model.GrantSupportSubscriptionWithMessage(id, c.GetInt("id"), c.GetInt("role"), request.PlanId, note)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(conversation.UserId, model.LogTypeManage, note, auditOperatorInfo(c))
	common.ApiSuccess(c, message)
}

func AdminCompleteSupportOrder(c *gin.Context) {
	messageId, err := strconv.Atoi(c.Param("id"))
	if err != nil || messageId <= 0 {
		common.ApiErrorMsg(c, "消息 ID 无效")
		return
	}
	var message model.SupportMessage
	if err := model.DB.First(&message, messageId).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if message.Kind != model.SupportMessageOrderQuote || message.OrderType != model.SupportOrderTopUp || message.OrderStatus != common.TopUpStatusPending || message.OrderTradeNo == "" {
		common.ApiErrorMsg(c, "该订单当前不可补单")
		return
	}
	if err := model.ManualCompleteTopUp(message.OrderTradeNo, c.ClientIP()); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.RefreshSupportOrderQuote(message.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"message_id": message.Id, "status": common.TopUpStatusSuccess})
}

func SupportImageResponse(message *model.SupportMessage) {
	if message == nil {
		return
	}
	if message.ImageData == "" && len(message.ImageDataBlob) > 0 {
		message.ImageData = base64.StdEncoding.EncodeToString(message.ImageDataBlob)
	}
	if message.ImageData == "" {
		return
	}
	if strings.HasPrefix(message.ImageData, "data:") {
		return
	}
	mime := message.ImageMime
	if mime == "" {
		mime = "image/jpeg"
	}
	message.ImageData = "data:" + mime + ";base64," + message.ImageData
}
