package console_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type ConsoleSetting struct {
	ApiInfo                  string `json:"api_info"`              // 控制台 API 信息 (JSON 数组字符串)
	UptimeKumaGroups         string `json:"uptime_kuma_groups"`    // Uptime Kuma 分组配置 (JSON 数组字符串)
	Announcements            string `json:"announcements"`         // 系统公告 (JSON 数组字符串)
	FAQ                      string `json:"faq"`                   // 常见问题 (JSON 数组字符串)
	ApiInfoEnabled           bool   `json:"api_info_enabled"`      // 是否启用 API 信息面板
	UptimeKumaEnabled        bool   `json:"uptime_kuma_enabled"`   // 是否启用 Uptime Kuma 面板
	AnnouncementsEnabled     bool   `json:"announcements_enabled"` // 是否启用系统公告面板
	FAQEnabled               bool   `json:"faq_enabled"`           // 是否启用常见问答面板
	BackgroundImage          string `json:"background_image"`
	BackgroundBlurOpacity    int    `json:"background_blur_opacity"`
	DefaultTheme             string `json:"default_theme"`
	DefaultThemePreset       string `json:"default_theme_preset"`
	DefaultThemeFont         string `json:"default_theme_font"`
	DefaultThemeRadius       string `json:"default_theme_radius"`
	DefaultThemeScale        string `json:"default_theme_scale"`
	DefaultSidebarVariant    string `json:"default_sidebar_variant"`
	DefaultSidebarLayout     string `json:"default_sidebar_layout"`
	DefaultContentLayout     string `json:"default_content_layout"`
	DefaultDirection         string `json:"default_direction"`
	ModelSquareDefaultView   string `json:"model_square_default_view"`
	ModelSquareCardPageSize  int    `json:"model_square_card_page_size"`
	ModelSquareTablePageSize int    `json:"model_square_table_page_size"`
	ModelSquareVisibleGroups string `json:"model_square_visible_groups"`
	GroupAccessRules         string `json:"group_access_rules"`
	HomepageStyle            string `json:"homepage_style"`
	HomepagePresetTitleMode  string `json:"homepage_preset_title_mode"`
	HomepagePresetSLAEnabled bool   `json:"homepage_preset_sla_enabled"`
	HomepagePresetSLAText    string `json:"homepage_preset_sla_text"`
	SPAMetaDescription       string `json:"spa_meta_description"`
	SPAMetaOGType            string `json:"spa_meta_og_type"`
	SPAMetaOGDescription     string `json:"spa_meta_og_description"`
	HideUpstreamRequestID    bool   `json:"hide_upstream_request_id"`
}

// 默认配置
var defaultConsoleSetting = ConsoleSetting{
	ApiInfo:                  "",
	UptimeKumaGroups:         "",
	Announcements:            "",
	FAQ:                      "",
	ApiInfoEnabled:           true,
	UptimeKumaEnabled:        true,
	AnnouncementsEnabled:     true,
	FAQEnabled:               true,
	BackgroundBlurOpacity:    40,
	DefaultTheme:             "system",
	DefaultThemePreset:       "default",
	DefaultThemeFont:         "default",
	DefaultThemeRadius:       "default",
	DefaultThemeScale:        "default",
	DefaultSidebarVariant:    "inset",
	DefaultSidebarLayout:     "expanded",
	DefaultContentLayout:     "full",
	DefaultDirection:         "ltr",
	ModelSquareDefaultView:   "card",
	ModelSquareCardPageSize:  18,
	ModelSquareTablePageSize: 20,
	ModelSquareVisibleGroups: "[]",
	GroupAccessRules:         "[]",
	HomepageStyle:            "default",
	HomepagePresetTitleMode:  "i18n",
	HomepagePresetSLAEnabled: true,
	HomepagePresetSLAText:    "99% SLA guarantee",
	SPAMetaDescription:       "Unified AI API gateway and admin dashboard.",
	SPAMetaOGType:            "website",
	SPAMetaOGDescription:     "Unified AI API gateway and admin dashboard.",
	HideUpstreamRequestID:    false,
}

// 全局实例
var consoleSetting = defaultConsoleSetting

func init() {
	// 注册到全局配置管理器，键名为 console_setting
	config.GlobalConfig.Register("console_setting", &consoleSetting)
}

// GetConsoleSetting 获取 ConsoleSetting 配置实例
func GetConsoleSetting() *ConsoleSetting {
	return &consoleSetting
}

type AppearanceSetting struct {
	BackgroundImage          string `json:"background_image"`
	BackgroundBlurOpacity    int    `json:"background_blur_opacity"`
	DefaultTheme             string `json:"default_theme"`
	DefaultThemePreset       string `json:"default_theme_preset"`
	DefaultThemeFont         string `json:"default_theme_font"`
	DefaultThemeRadius       string `json:"default_theme_radius"`
	DefaultThemeScale        string `json:"default_theme_scale"`
	DefaultSidebarVariant    string `json:"default_sidebar_variant"`
	DefaultSidebarLayout     string `json:"default_sidebar_layout"`
	DefaultContentLayout     string `json:"default_content_layout"`
	DefaultDirection         string `json:"default_direction"`
	ModelSquareDefaultView   string `json:"model_square_default_view"`
	ModelSquareCardPageSize  int    `json:"model_square_card_page_size"`
	ModelSquareTablePageSize int    `json:"model_square_table_page_size"`
}

type SPAMetaSetting struct {
	Description   string `json:"description"`
	OGType        string `json:"og_type"`
	OGDescription string `json:"og_description"`
}

type HomepageSetting struct {
	Style            string `json:"style"`
	PresetTitleMode  string `json:"preset_title_mode"`
	PresetSLAEnabled bool   `json:"preset_sla_enabled"`
	PresetSLAText    string `json:"preset_sla_text"`
}

func GetHomepageSetting() HomepageSetting {
	return HomepageSetting{
		Style:            consoleSetting.HomepageStyle,
		PresetTitleMode:  consoleSetting.HomepagePresetTitleMode,
		PresetSLAEnabled: consoleSetting.HomepagePresetSLAEnabled,
		PresetSLAText:    consoleSetting.HomepagePresetSLAText,
	}
}

func GetAppearanceSetting() AppearanceSetting {
	return AppearanceSetting{
		BackgroundImage:          consoleSetting.BackgroundImage,
		BackgroundBlurOpacity:    consoleSetting.BackgroundBlurOpacity,
		DefaultTheme:             consoleSetting.DefaultTheme,
		DefaultThemePreset:       consoleSetting.DefaultThemePreset,
		DefaultThemeFont:         consoleSetting.DefaultThemeFont,
		DefaultThemeRadius:       consoleSetting.DefaultThemeRadius,
		DefaultThemeScale:        consoleSetting.DefaultThemeScale,
		DefaultSidebarVariant:    consoleSetting.DefaultSidebarVariant,
		DefaultSidebarLayout:     consoleSetting.DefaultSidebarLayout,
		DefaultContentLayout:     consoleSetting.DefaultContentLayout,
		DefaultDirection:         consoleSetting.DefaultDirection,
		ModelSquareDefaultView:   consoleSetting.ModelSquareDefaultView,
		ModelSquareCardPageSize:  consoleSetting.ModelSquareCardPageSize,
		ModelSquareTablePageSize: consoleSetting.ModelSquareTablePageSize,
	}
}

func GetSPAMetaSetting() SPAMetaSetting {
	return SPAMetaSetting{
		Description:   consoleSetting.SPAMetaDescription,
		OGType:        consoleSetting.SPAMetaOGType,
		OGDescription: consoleSetting.SPAMetaOGDescription,
	}
}

var ogTypePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._:-]{0,63}$`)

func validatePublicImageURL(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 2048 {
		return fmt.Errorf("%s URL is too long", label)
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		if _, err := url.ParseRequestURI(value); err == nil {
			return nil
		}
		return fmt.Errorf("%s contains invalid URL characters", label)
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must use an absolute HTTP(S) URL or a root-relative path", label)
	}
	return nil
}

func validateEnum(value string, allowed ...string) error {
	for _, item := range allowed {
		if value == item {
			return nil
		}
	}
	return fmt.Errorf("invalid value %q", value)
}

// ValidatePublicOption validates options that are consumed by the public SPA.
// Unknown keys are intentionally ignored so older deployments can add fields
// without making this validator a compatibility bottleneck.
func ValidatePublicOption(key, value string) error {
	if key == "Logo" {
		return validatePublicImageURL("logo", value)
	}
	if !strings.HasPrefix(key, "console_setting.") {
		return nil
	}
	name := strings.TrimPrefix(key, "console_setting.")
	switch name {
	case "background_image":
		return validatePublicImageURL("background image", value)
	case "background_blur_opacity":
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed < 0 || parsed > 100 {
			return fmt.Errorf("background blur opacity must be between 0 and 100")
		}
	case "default_theme":
		return validateEnum(value, "system", "light", "dark")
	case "default_theme_preset":
		return validateEnum(value, "default", "anthropic", "simple-large", "underground", "rose-garden", "lake-view", "sunset-glow", "forest-whisper", "ocean-breeze", "lavender-dream")
	case "default_theme_font":
		return validateEnum(value, "default", "sans", "serif")
	case "default_theme_radius":
		return validateEnum(value, "default", "none", "sm", "md", "lg", "xl")
	case "default_theme_scale":
		return validateEnum(value, "default", "sm", "lg", "xl")
	case "default_sidebar_variant":
		return validateEnum(value, "inset", "floating", "sidebar")
	case "default_sidebar_layout":
		return validateEnum(value, "expanded", "icon", "offcanvas")
	case "default_content_layout":
		return validateEnum(value, "full", "centered")
	case "default_direction":
		return validateEnum(value, "ltr", "rtl")
	case "model_square_default_view":
		return validateEnum(value, "card", "table")
	case "model_square_card_page_size":
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed < 6 || parsed > 96 || parsed%6 != 0 {
			return fmt.Errorf("model square card page size must be a multiple of 6 between 6 and 96")
		}
	case "model_square_table_page_size":
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || parsed < 5 || parsed > 100 {
			return fmt.Errorf("model square table page size must be between 5 and 100")
		}
	case "homepage_style":
		return validateEnum(value, "default", "custom", "preset-1")
	case "homepage_preset_title_mode":
		return validateEnum(value, "i18n", "english")
	case "homepage_preset_sla_text":
		if len(value) > 120 {
			return fmt.Errorf("homepage SLA text is too long")
		}
	case "spa_meta_description", "spa_meta_og_description":
		if len(value) > 500 {
			return fmt.Errorf("SPA meta description is too long")
		}
	case "spa_meta_og_type":
		if !ogTypePattern.MatchString(value) {
			return fmt.Errorf("invalid Open Graph type")
		}
	}
	return nil
}
