package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const OrderTypeSubscription = "subscription"

// Admin history merges gateway subscription orders with top-ups before
// paginating. A subscription appears exactly once, even after the callback
// writes its legacy top-up history row. Never expose checkout payloads here.
func queryAdminBillingOrders(keyword string, page *common.PageInfo) (records []*TopUp, total int64, err error) {
	keyword = strings.TrimSpace(keyword)
	if len(keyword) > 255 {
		return nil, 0, common.NewMessage("Order search keyword is too long")
	}
	pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword)) + "%"
	if page.GetStartIdx() < 0 || page.GetPageSize() <= 0 {
		return nil, 0, common.NewMessage("Invalid pagination")
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		search := func(q *gorm.DB, alias string) *gorm.DB {
			if keyword == "" {
				return q
			}
			idType := "TEXT"
			if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
				idType = "CHAR"
			}
			return q.Where("(LOWER("+alias+".trade_no) LIKE ? ESCAPE '!' OR CAST("+alias+".user_id AS "+idType+") LIKE ? ESCAPE '!' OR EXISTS (?))",
				pattern, pattern, tx.Model(&User{}).Select("1").Where("users.id = "+alias+".user_id AND LOWER(users.username) LIKE ? ESCAPE '!'", pattern))
		}
		topups := search(tx.Table("top_ups AS topups").Select("topups.id, topups.create_time, 0 AS is_subscription"), "topups")
		subscriptions := search(tx.Table("subscription_orders AS subscriptions").
			Select("subscriptions.id, subscriptions.create_time, 1 AS is_subscription").
			Where("NOT EXISTS (?)", tx.Model(&TopUp{}).Select("1").Where("top_ups.trade_no = subscriptions.trade_no")), "subscriptions")
		query := tx.Table("(? UNION ALL ?) AS orders", topups, subscriptions)
		countQuery := query
		if keyword != "" {
			countQuery = tx.Table("(?) AS bounded_orders", query.Session(&gorm.Session{}).Select("id").Limit(searchTopUpCountHardLimit))
		}
		if err := countQuery.Count(&total).Error; err != nil {
			return err
		}
		var rows []struct {
			ID             int
			IsSubscription int
		}
		offset := page.GetStartIdx()
		if int64(offset) >= total {
			return nil
		}
		limit := min(page.GetPageSize(), int(total)-offset)
		if err := query.Select("id, is_subscription").Order("create_time DESC, is_subscription ASC, id DESC").
			Limit(limit).Offset(offset).Scan(&rows).Error; err != nil {
			return err
		}
		var topupIDs, subscriptionIDs []int
		for _, row := range rows {
			if row.IsSubscription == 1 {
				subscriptionIDs = append(subscriptionIDs, row.ID)
			} else {
				topupIDs = append(topupIDs, row.ID)
			}
		}
		byID := make(map[int]*TopUp, len(rows))
		if len(topupIDs) > 0 {
			var topups []*TopUp
			if err := tx.Find(&topups, topupIDs).Error; err != nil {
				return err
			}
			var mirrors []SubscriptionOrder
			if err := tx.Select("trade_no").Where("trade_no IN (?)", tx.Model(&TopUp{}).Select("trade_no").Where("id IN ?", topupIDs)).Find(&mirrors).Error; err != nil {
				return err
			}
			mirrored := make(map[string]bool, len(mirrors))
			for _, order := range mirrors {
				mirrored[order.TradeNo] = true
			}
			for _, topup := range topups {
				if mirrored[topup.TradeNo] {
					topup.OrderType = OrderTypeSubscription
				}
				byID[topup.Id] = topup
			}
		}
		if len(subscriptionIDs) > 0 {
			var subscriptions []SubscriptionOrder
			if err := tx.Select("id", "user_id", "money", "payment_currency", "trade_no", "payment_method", "payment_provider", "epay_gateway_id", "create_time", "complete_time", "status").Find(&subscriptions, subscriptionIDs).Error; err != nil {
				return err
			}
			for _, sub := range subscriptions {
				byID[-sub.Id] = &TopUp{Id: -sub.Id, UserId: sub.UserId, Money: sub.Money, PaymentCurrency: sub.PaymentCurrency,
					TradeNo: sub.TradeNo, PaymentMethod: sub.PaymentMethod, PaymentProvider: sub.PaymentProvider,
					EpayGatewayID: sub.EpayGatewayID, CreateTime: sub.CreateTime, CompleteTime: sub.CompleteTime,
					Status: sub.Status, OrderType: OrderTypeSubscription}
			}
		}
		for _, row := range rows {
			id := row.ID
			if row.IsSubscription == 1 {
				id = -id
			}
			if record := byID[id]; record != nil {
				records = append(records, record)
			}
		}
		return nil
	})
	return
}
