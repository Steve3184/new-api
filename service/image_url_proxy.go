/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const imageURLProxyPathPrefix = "/v1/artifacts/"

type imageURLProxyEntry struct {
	URL       string
	ExpiresAt time.Time
}

var imageURLProxyStore = struct {
	sync.Mutex
	entries map[string]imageURLProxyEntry
}{entries: make(map[string]imageURLProxyEntry)}

func shouldProxyGeneratedImageURLs(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	typeID := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if typeID != constant.ChannelTypeOpenAI && typeID != constant.ChannelTypeGemini {
		return false
	}
	settings, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
	if !ok || !settings.ProxyImageURLs {
		return false
	}
	path := c.Request.URL.Path
	return path == "/v1/images/generations" || path == "/v1/images/edits" ||
		path == "/pg/images/generations" || path == "/pg/images/edits" ||
		(strings.HasPrefix(path, "/v1beta/models/") &&
			(strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent")))
}

func proxyImageResponseOrOriginal(c *gin.Context, body []byte) []byte {
	if !shouldProxyGeneratedImageURLs(c) {
		return body
	}
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return body
	}
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		return body
	}
	index := 0
	changed := rewriteGeneratedImageURLs(c, payload, requestID, &index)
	if !changed {
		return body
	}
	rewritten, err := common.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func rewriteGeneratedImageURLs(c *gin.Context, value any, requestID string, index *int) bool {
	switch current := value.(type) {
	case []any:
		changed := false
		for _, item := range current {
			if rewriteGeneratedImageURLs(c, item, requestID, index) {
				changed = true
			}
		}
		return changed
	case map[string]any:
		changed := false
		for _, key := range []string{"url", "fileUri", "file_uri"} {
			if raw, ok := current[key].(string); ok && isProxyableImageURL(raw) {
				current[key] = saveImageURLProxy(c, requestID, *index, raw)
				changed = true
				(*index)++
			}
		}
		if raw, ok := current["image_url"].(string); ok && isProxyableImageURL(raw) {
			current["image_url"] = saveImageURLProxy(c, requestID, *index, raw)
			changed = true
			(*index)++
		}
		for _, child := range current {
			if rewriteGeneratedImageURLs(c, child, requestID, index) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func isProxyableImageURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func saveImageURLProxy(c *gin.Context, requestID string, index int, originalURL string) string {
	artifactID := requestID
	if index > 0 {
		artifactID = fmt.Sprintf("%s-%d", requestID, index+1)
	}
	imageURLProxyStore.Lock()
	imageURLProxyStore.entries[artifactID] = imageURLProxyEntry{URL: originalURL, ExpiresAt: time.Now().Add(30 * time.Minute)}
	imageURLProxyStore.Unlock()
	baseAddress := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if baseAddress == "" && c != nil && c.Request != nil {
		scheme := "http"
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil {
			scheme = "https"
		}
		baseAddress = scheme + "://" + c.Request.Host
	}
	if parsed, err := url.Parse(baseAddress); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + imageURLProxyPathPrefix + artifactID + ".png"
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	return imageURLProxyPathPrefix + artifactID + ".png"
}

// ImageURLProxy serves a short-lived URL that hides the upstream image URL.
func ImageURLProxy(c *gin.Context) {
	artifactID := strings.TrimSuffix(strings.TrimSpace(c.Param("artifact_id")), ".png")
	imageURLProxyStore.Lock()
	entry, ok := imageURLProxyStore.entries[artifactID]
	if ok && time.Now().After(entry.ExpiresAt) {
		delete(imageURLProxyStore.entries, artifactID)
		ok = false
	}
	imageURLProxyStore.Unlock()
	if !ok || !isProxyableImageURL(entry.URL) {
		c.Status(http.StatusNotFound)
		return
	}
	if err := ValidateSSRFProtectedFetchURL(entry.URL); err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, entry.URL, nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	response, err := GetSSRFProtectedHTTPClient().Do(request)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer CloseResponseBodyGracefully(response)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		c.Status(http.StatusBadGateway)
		return
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Cache-Control", "private, max-age=1800")
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, response.Body)
}

func RewriteGeneratedImageURLs(c *gin.Context, body []byte) []byte {
	return proxyImageResponseOrOriginal(c, body)
}
