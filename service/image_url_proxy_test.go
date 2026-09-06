/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteGeneratedImageURLsUsesConfiguredAbsoluteAddress(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://gateway.example/prefix"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/v1/images/generations", nil)
	common.SetContextKey(context, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(context, constant.ContextKeyChannelSetting, dto.ChannelSettings{ProxyImageURLs: true})
	context.Set(common.RequestIdKey, "request-123")

	body, err := common.Marshal(map[string]any{
		"data": []any{
			map[string]any{"url": "https://upstream.example/one.png"},
			map[string]any{"url": "https://upstream.example/two.png", "nested": map[string]any{"file_uri": "https://upstream.example/three.png"}},
			map[string]any{"image_url": map[string]any{"url": "https://upstream.example/four.png"}},
		},
	})
	require.NoError(t, err)

	rewritten := RewriteGeneratedImageURLs(context, body)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rewritten, &payload))
	data := payload["data"].([]any)
	assert.Equal(t, "https://gateway.example/prefix/v1/artifacts/request-123.png", data[0].(map[string]any)["url"])
	assert.Equal(t, "https://gateway.example/prefix/v1/artifacts/request-123-2.png", data[1].(map[string]any)["url"])
	assert.Equal(t, "https://gateway.example/prefix/v1/artifacts/request-123-3.png", data[1].(map[string]any)["nested"].(map[string]any)["file_uri"])
	assert.Equal(t, "https://gateway.example/prefix/v1/artifacts/request-123-4.png", data[2].(map[string]any)["image_url"].(map[string]any)["url"])
}

func TestRewriteGeneratedImageURLsSkipsUnsupportedChannels(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/v1/images/generations", nil)
	common.SetContextKey(context, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(context, constant.ContextKeyChannelSetting, dto.ChannelSettings{ProxyImageURLs: true})
	context.Set(common.RequestIdKey, "request-123")

	body := []byte(`{"data":[{"url":"https://upstream.example/image.png"}]}`)
	assert.Equal(t, body, RewriteGeneratedImageURLs(context, body))
}
