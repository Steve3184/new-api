package model_setting

import (
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ChatCompletionsToResponsesPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

type ResponsesToChatCompletionsPolicy struct {
	Enabled       bool     `json:"enabled"`
	AllChannels   bool     `json:"all_channels"`
	ChannelIDs    []int    `json:"channel_ids,omitempty"`
	ChannelTypes  []int    `json:"channel_types,omitempty"`
	ModelPatterns []string `json:"model_patterns,omitempty"`
}

func (p ResponsesToChatCompletionsPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

func (p ChatCompletionsToResponsesPolicy) IsChannelEnabled(channelID int, channelType int) bool {
	if !p.Enabled {
		return false
	}
	if p.AllChannels {
		return true
	}

	if channelID > 0 && len(p.ChannelIDs) > 0 && slices.Contains(p.ChannelIDs, channelID) {
		return true
	}
	if channelType > 0 && len(p.ChannelTypes) > 0 && slices.Contains(p.ChannelTypes, channelType) {
		return true
	}
	return false
}

type GlobalSettings struct {
	PassThroughRequestEnabled        bool                                    `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist           []string                                `json:"thinking_model_blacklist"`
	ChatCompletionsToResponsesPolicy ChatCompletionsToResponsesPolicy        `json:"chat_completions_to_responses_policy"`
	ResponsesToChatCompletionsPolicy ResponsesToChatCompletionsPolicy        `json:"responses_to_chat_completions_policy"`
	EffortModelRoutes                map[string]map[string]string             `json:"effort_model_routes"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	ChatCompletionsToResponsesPolicy: ChatCompletionsToResponsesPolicy{
		Enabled:     false,
		AllChannels: true,
	},
	ResponsesToChatCompletionsPolicy: ResponsesToChatCompletionsPolicy{
		Enabled:     false,
		AllChannels: true,
	},
	EffortModelRoutes: map[string]map[string]string{},
}

func (s *GlobalSettings) RouteModelByEffort(modelName, effort string) (string, bool) {
	modelName = strings.TrimSpace(modelName)
	effort = strings.ToLower(strings.TrimSpace(effort))
	if modelName == "" || effort == "" {
		return modelName, false
	}
	efforts := s.EffortModelRoutes[modelName]
	routed, ok := efforts[effort]
	if !ok || strings.TrimSpace(routed) == "" {
		return modelName, false
	}
	return strings.TrimSpace(routed), true
}

func ValidateEffortModelRoutes(value string) error {
	var routes map[string]map[string]string
	if err := common.UnmarshalJsonStr(value, &routes); err != nil {
		return fmt.Errorf("effort model routes must be a JSON object: %w", err)
	}
	for modelName, efforts := range routes {
		if strings.TrimSpace(modelName) == "" {
			return fmt.Errorf("effort route model name cannot be empty")
		}
		for effort, target := range efforts {
			if strings.TrimSpace(effort) == "" || strings.TrimSpace(target) == "" {
				return fmt.Errorf("effort route entries require non-empty effort and target")
			}
		}
	}
	return nil
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}
