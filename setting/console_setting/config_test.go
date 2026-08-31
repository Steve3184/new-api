package console_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePublicOption(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		{name: "absolute background", key: "console_setting.background_image", value: "https://cdn.example.com/background.jpg"},
		{name: "relative background", key: "console_setting.background_image", value: "/_custom/img/background.jpg"},
		{name: "reject unsafe background", key: "console_setting.background_image", value: "javascript:alert(1)", wantErr: true},
		{name: "background blur opacity", key: "console_setting.background_blur_opacity", value: "60"},
		{name: "reject background blur opacity above range", key: "console_setting.background_blur_opacity", value: "101", wantErr: true},
		{name: "relative logo", key: "Logo", value: "/_custom/img/logo.png"},
		{name: "reject unsafe logo", key: "Logo", value: "data:image/svg+xml,test", wantErr: true},
		{name: "valid preset", key: "console_setting.default_theme_preset", value: "anthropic"},
		{name: "invalid preset", key: "console_setting.default_theme_preset", value: "unknown", wantErr: true},
		{name: "three-column card page", key: "console_setting.model_square_card_page_size", value: "18"},
		{name: "reject incomplete card row", key: "console_setting.model_square_card_page_size", value: "20", wantErr: true},
		{name: "table page range", key: "console_setting.model_square_table_page_size", value: "100"},
		{name: "invalid og type", key: "console_setting.spa_meta_og_type", value: "website\" onload=\"x", wantErr: true},
		{name: "ignore unrelated option", key: "SystemName", value: "anything"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePublicOption(tt.key, tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPublicSettingsDefaults(t *testing.T) {
	appearance := GetAppearanceSetting()
	meta := GetSPAMetaSetting()
	homepage := GetHomepageSetting()

	assert.Equal(t, "system", appearance.DefaultTheme)
	assert.Equal(t, 40, appearance.BackgroundBlurOpacity)
	assert.Equal(t, "card", appearance.ModelSquareDefaultView)
	assert.Equal(t, 18, appearance.ModelSquareCardPageSize)
	assert.Equal(t, 20, appearance.ModelSquareTablePageSize)
	assert.Equal(t, "website", meta.OGType)
	assert.Equal(t, "default", homepage.Style)
	assert.Equal(t, "i18n", homepage.PresetTitleMode)
	assert.True(t, homepage.PresetSLAEnabled)
	assert.Equal(t, "99% SLA guarantee", homepage.PresetSLAText)
}
