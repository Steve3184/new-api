package service

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskArtifactAccessBindsTaskAndKey(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "task-artifact-access-test-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	access, err := IssueTaskArtifactAccess("task-1", "video-main")
	require.NoError(t, err)
	assert.Len(t, access, 43)
	assert.NotContains(t, access, ".")
	assert.True(t, VerifyTaskArtifactAccess(access, "task-1", "video-main"))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-2", "video-main"))
	assert.False(t, VerifyTaskArtifactAccess(access, "task-1", "video-other"))
	assert.False(t, VerifyTaskArtifactAccess(access+"x", "task-1", "video-main"))

	common.CryptoSecret = "another-node-secret"
	assert.False(t, VerifyTaskArtifactAccess(access, "task-1", "video-main"))
}

func TestBuildTaskArtifactContentURLUsesConfiguredAddressAndPreservesPrefix(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	previousServerAddress := system_setting.ServerAddress
	common.CryptoSecret = "task-artifact-url-test-secret"
	system_setting.TaskPublicAddress = "https://media.example/gateway/prefix/"
	system_setting.ServerAddress = "https://fallback.invalid"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
		system_setting.ServerAddress = previousServerAddress
	})

	contentURL, err := BuildTaskArtifactContentURL("task-public", "video-main")
	require.NoError(t, err)
	parsed, err := url.Parse(contentURL)
	require.NoError(t, err)
	assert.Equal(t, "media.example", parsed.Host)
	assert.Equal(t, "/gateway/prefix/v1/tasks/task-public/artifacts/video-main/content", parsed.Path)
	assert.True(t, VerifyTaskArtifactAccess(
		parsed.Query().Get(TaskArtifactAccessQueryParameter),
		"task-public",
		"video-main",
	))
}

func TestBuildTaskArtifactContentURLFallsBackOnlyToServerAddress(t *testing.T) {
	previousSecret := common.CryptoSecret
	previousPublicAddress := system_setting.TaskPublicAddress
	previousServerAddress := system_setting.ServerAddress
	common.CryptoSecret = "task-artifact-fallback-test-secret"
	system_setting.TaskPublicAddress = ""
	system_setting.ServerAddress = "https://gateway.example/root"
	t.Cleanup(func() {
		common.CryptoSecret = previousSecret
		system_setting.TaskPublicAddress = previousPublicAddress
		system_setting.ServerAddress = previousServerAddress
	})

	contentURL, err := BuildTaskArtifactContentURL("task-fallback", "audio")
	require.NoError(t, err)
	assert.Contains(t, contentURL, "https://gateway.example/root/v1/tasks/task-fallback/artifacts/audio/content")

	system_setting.TaskPublicAddress = "not-a-url"
	_, err = BuildTaskArtifactContentURL("task-fallback", "audio")
	assert.Error(t, err)
}

func TestValidateTaskArtifactBaseURLOnlyAcceptsSafeAbsoluteHTTPURLs(t *testing.T) {
	for _, valid := range []string{
		"http://localhost:3000",
		"https://gateway.example",
		"https://gateway.example/prefix/path/",
	} {
		assert.NoError(t, ValidateTaskArtifactBaseURL(valid), valid)
	}
	for _, invalid := range []string{
		"",
		"/relative",
		"ftp://gateway.example",
		"https://user:secret@gateway.example",
		"https://gateway.example/path?tenant=1",
		"https://gateway.example/path#fragment",
		" https://gateway.example",
		"https://gateway.example ",
	} {
		assert.Error(t, ValidateTaskArtifactBaseURL(invalid), invalid)
	}
}

func TestTaskInputMediaValidationAndLifetime(t *testing.T) {
	previousSecret, previousAddress := common.CryptoSecret, system_setting.TaskPublicAddress
	common.CryptoSecret, system_setting.TaskPublicAddress = "input-media-test-secret", "https://media.example"
	t.Cleanup(func() { common.CryptoSecret, system_setting.TaskPublicAddress = previousSecret, previousAddress })
	png := []byte("\x89PNG\r\n\x1a\nimage-payload")
	mp4 := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	webm := []byte("\x1a\x45\xdf\xa3\x87\x42\x82\x84webm")
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt "), make([]byte, 32)...)
	for _, tc := range []struct {
		name, value, kind, mimeType string
		valid                       bool
	}{
		{"image data URI", "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), "image", "image/png", true},
		{"bare audio", base64.StdEncoding.EncodeToString(wav), "audio", "audio/wav", true},
		{"bare video MP4", base64.StdEncoding.EncodeToString(mp4), "video", "video/mp4", true},
		{"video WebM URI", "data:video/webm;base64," + base64.StdEncoding.EncodeToString(webm), "video", "video/webm", true},
		{"audio MP4 URI", "data:audio/mp4;base64," + base64.StdEncoding.EncodeToString(mp4), "audio", "audio/mp4", true},
		{"bare MP4 is not audio", base64.StdEncoding.EncodeToString(mp4), "audio", "", false},
		{"video URI is not audio", "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(mp4), "audio", "", false},
		{"audio URI is not video", "data:audio/mp4;base64," + base64.StdEncoding.EncodeToString(mp4), "video", "", false},
		{"wrong video container", "data:video/webm;base64," + base64.StdEncoding.EncodeToString(mp4), "video", "", false},
		{"image is not video", base64.StdEncoding.EncodeToString(png), "video", "", false},
		{"wrong MIME", "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(png), "image", "", false},
		{"wrong kind", base64.StdEncoding.EncodeToString(png), "audio", "", false},
		{"invalid base64", "data:image/png;base64,@@@@", "image", "", false},
		{"active content", "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg/>")), "image", "", false},
		{"empty", "", "audio", "", false},
		{"oversized", strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxTaskInputMediaBytes)+4), "audio", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, mimeType, err := DecodeTaskInputMedia(tc.value, tc.kind)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, data)
			assert.Equal(t, tc.mimeType, mimeType)
		})
	}
	urls, err := StoreTaskInputMediaBatch(1, [][]byte{png}, []string{"image/png"})
	require.NoError(t, err)
	parsed, err := url.Parse(urls[0])
	require.NoError(t, err)
	parts := strings.Split(parsed.Path, "/")
	id, key := parts[3], parts[4]
	assert.True(t, VerifyTaskArtifactAccess(parsed.Query().Get("access"), id, key))
	taskInputs.Lock()
	item := taskInputs.items[id]
	assert.WithinDuration(t, time.Now().Add(time.Hour), item.expires, 2*time.Second)
	item.expires = time.Now().Add(-time.Second)
	taskInputs.items[id] = item
	taskInputs.Unlock()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "key", Value: id}, {Key: "artifact_key", Value: key}}
	c.Request = httptest.NewRequest(http.MethodGet, urls[0], nil)
	ServeTaskInputMedia(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	// A new batch also removes expired bytes, and exhausted capacity cannot evict
	// still-valid inputs or publish a partial batch.
	taskInputs.Lock()
	delete(taskInputs.items, id)
	taskInputs.bytes -= len(item.data)
	savedBytes := taskInputs.bytes
	taskInputs.bytes = maxTaskInputMediaCapacity
	taskInputs.Unlock()
	_, err = StoreTaskInputMediaBatch(1, [][]byte{png}, []string{"image/png"})
	assert.ErrorContains(t, err, "capacity")
	taskInputs.Lock()
	taskInputs.bytes = savedBytes
	taskInputs.Unlock()
}
