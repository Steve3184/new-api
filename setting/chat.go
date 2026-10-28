package setting

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

var Chats = []map[string]string{
	//{
	//	"ChatGPT Next Web 官方示例": "https://app.nextchat.dev/#/?settings={\"key\":\"{key}\",\"url\":\"{address}\"}",
	//},
	{
		"Cherry Studio": "cherrystudio://providers/api-keys?v=1&data={cherryConfig}",
	},
	{
		"AionUI": "aionui://provider/add?v=1&data={aionuiConfig}",
	},
	{
		"流畅阅读": "fluentread",
	},
	{
		"CC Switch": "ccswitch",
	},
	{
		"DeepChat": "deepchat://provider/install?v=1&data={deepchatConfig}",
	},
	{
		"AQBot": "aqbot://providers?{aqbotConfig}",
	},
	{
		"Lobe Chat 官方示例": "https://chat-preview.lobehub.com/?settings={\"keyVaults\":{\"openai\":{\"apiKey\":\"{key}\",\"baseURL\":\"{address}/v1\"}}}",
	},
	{
		"AI as Workspace": "https://aiaw.app/set-provider?provider={\"type\":\"openai\",\"settings\":{\"apiKey\":\"{key}\",\"baseURL\":\"{address}/v1\",\"compatibility\":\"strict\"}}",
	},
	{
		"AMA 问天": "ama://set-api-key?server={address}&key={key}",
	},
	{
		"OpenCat": "opencat://team/join?domain={address}&token={key}",
	},
}

func UpdateChatsByJsonString(jsonString string) error {
	Chats = make([]map[string]string, 0)
	return json.Unmarshal([]byte(jsonString), &Chats)
}

func Chats2JsonString() string {
	jsonBytes, err := json.Marshal(Chats)
	if err != nil {
		common.SysLog(common.LogText("error marshalling chats: %s", err.Error()))
		return "[]"
	}
	return string(jsonBytes)
}

// ChatPresetAutoKey is the template for the API key created on first use of a chat preset.
type ChatPresetAutoKey struct {
	Enabled             bool                `json:"enabled"`
	Name                string              `json:"name"`
	RemainQuota         int                 `json:"remain_quota"`
	UnlimitedQuota      bool                `json:"unlimited_quota"`
	ModelLimitsEnabled  bool                `json:"model_limits_enabled"`
	ModelLimits         string              `json:"model_limits"`
	AllowIps            string              `json:"allow_ips"`
	AllowUserQuotaQuery bool                `json:"allow_user_quota_query"`
	Group               string              `json:"group"`
	AutoGroups          []string            `json:"auto_groups"`
	AutoRoutes          map[string][]string `json:"auto_routes"`
	CrossGroupRetry     bool                `json:"cross_group_retry"`
}

// ChatPresetSetting holds the per-preset options keyed by the preset name in Chats.
type ChatPresetSetting struct {
	StandaloneMenu bool               `json:"standalone_menu"`
	AutoKey        *ChatPresetAutoKey `json:"auto_key,omitempty"`
}

// ChatPresetPublicSetting is the part of a preset setting that /api/status may expose;
// the key template stays visible to administrators only.
type ChatPresetPublicSetting struct {
	StandaloneMenu bool `json:"standalone_menu"`
	AutoKey        bool `json:"auto_key"`
}

const (
	maxChatPresetSettings  = 100
	maxChatPresetKeyLength = 50
)

var (
	chatPresetSettingsMu sync.RWMutex
	chatPresetSettings   = map[string]ChatPresetSetting{}
)

// ChatPresetAutoKeyName returns the token name used for a preset: the configured
// name, or app-<preset name> cut to the token name limit.
func ChatPresetAutoKeyName(presetName string, autoKey ChatPresetAutoKey) string {
	if name := strings.TrimSpace(autoKey.Name); name != "" {
		return name
	}
	name := "app-" + strings.TrimSpace(presetName)
	for len(name) > maxChatPresetKeyLength {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func ParseChatPresetSettings(jsonString string) (map[string]ChatPresetSetting, error) {
	parsed := map[string]ChatPresetSetting{}
	if strings.TrimSpace(jsonString) == "" {
		return parsed, nil
	}
	if err := common.UnmarshalJsonStr(jsonString, &parsed); err != nil {
		return nil, common.NewMessage("Invalid chat preset settings")
	}
	if len(parsed) > maxChatPresetSettings {
		return nil, common.NewMessage("At most {{max}} chat preset settings are allowed", map[string]any{"max": maxChatPresetSettings})
	}
	result := make(map[string]ChatPresetSetting, len(parsed))
	for presetName, presetSetting := range parsed {
		presetName = strings.TrimSpace(presetName)
		if presetName == "" {
			return nil, common.NewMessage("Chat preset settings must use a preset name as the key")
		}
		if presetSetting.AutoKey != nil {
			autoKey := *presetSetting.AutoKey
			autoKey.Name = strings.TrimSpace(autoKey.Name)
			keyName := ChatPresetAutoKeyName(presetName, autoKey)
			if keyName == "" || len(keyName) > maxChatPresetKeyLength {
				return nil, common.NewMessage("The API key name for chat preset {{name}} must contain 1 to {{max}} bytes", map[string]any{"name": presetName, "max": maxChatPresetKeyLength})
			}
			if strings.Contains(keyName, "%") {
				return nil, common.NewMessage("The API key name for chat preset {{name}} cannot contain %", map[string]any{"name": presetName})
			}
			if autoKey.RemainQuota < 0 {
				return nil, common.NewMessage("The API key quota for chat preset {{name}} cannot be negative", map[string]any{"name": presetName})
			}
			autoKey.Group = strings.TrimSpace(autoKey.Group)
			autoKey.ModelLimits = strings.TrimSpace(autoKey.ModelLimits)
			autoKey.ModelLimitsEnabled = autoKey.ModelLimits != ""
			if autoKey.Group != "auto" {
				autoKey.AutoGroups = nil
				autoKey.AutoRoutes = nil
				autoKey.CrossGroupRetry = false
			}
			presetSetting.AutoKey = &autoKey
		}
		result[presetName] = presetSetting
	}
	return result, nil
}

func UpdateChatPresetSettingsByJsonString(jsonString string) error {
	parsed, err := ParseChatPresetSettings(jsonString)
	if err != nil {
		return err
	}
	chatPresetSettingsMu.Lock()
	chatPresetSettings = parsed
	chatPresetSettingsMu.Unlock()
	return nil
}

func ChatPresetSettings2JsonString() string {
	chatPresetSettingsMu.RLock()
	defer chatPresetSettingsMu.RUnlock()
	jsonBytes, err := common.Marshal(chatPresetSettings)
	if err != nil {
		common.SysLog(common.LogText("error marshalling chat preset settings: %s", err.Error()))
		return "{}"
	}
	return string(jsonBytes)
}

// GetChatPresetAutoKey returns the key template of a preset that still exists in Chats
// and has automatic key creation turned on.
func GetChatPresetAutoKey(presetName string) (ChatPresetAutoKey, bool) {
	presetExists := slices.ContainsFunc(Chats, func(entry map[string]string) bool {
		_, ok := entry[presetName]
		return ok
	})
	if !presetExists {
		return ChatPresetAutoKey{}, false
	}
	chatPresetSettingsMu.RLock()
	defer chatPresetSettingsMu.RUnlock()
	autoKey := chatPresetSettings[presetName].AutoKey
	if autoKey == nil || !autoKey.Enabled {
		return ChatPresetAutoKey{}, false
	}
	result := *autoKey
	result.AutoGroups = slices.Clone(autoKey.AutoGroups)
	result.AutoRoutes = maps.Clone(autoKey.AutoRoutes)
	return result, true
}

func GetPublicChatPresetSettings() map[string]ChatPresetPublicSetting {
	chatPresetSettingsMu.RLock()
	defer chatPresetSettingsMu.RUnlock()
	result := make(map[string]ChatPresetPublicSetting, len(chatPresetSettings))
	for presetName, presetSetting := range chatPresetSettings {
		result[presetName] = ChatPresetPublicSetting{
			StandaloneMenu: presetSetting.StandaloneMenu,
			AutoKey:        presetSetting.AutoKey != nil && presetSetting.AutoKey.Enabled,
		}
	}
	return result
}
