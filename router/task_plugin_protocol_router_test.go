package router

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHostProtocolRegistryDrivesProtocolRoutesOnce(t *testing.T) {
	engine := gin.New()
	SetTaskPluginProtocolRouter(engine)

	expected := []string{
		"POST /v1/responses",
		"GET /v1/responses/:response_id",
		"POST /v1/videos",
		"POST /v1/audio/speech",
		"POST /v1/audio/speech/tasks",
		"GET /v1/audio/speech/tasks/:task_id",
		"GET /v1/audio/speech/tasks/:task_id/content",
		"HEAD /v1/audio/speech/tasks/:task_id/content",
		"GET /v1/videos/:task_id",
		"GET /v1/videos/:task_id/content",
		"HEAD /v1/videos/:task_id/content",
		"POST /v1/images/generations",
		"POST /v1/images/edits",
	}
	actual := make([]string, 0, len(engine.Routes()))
	for _, route := range engine.Routes() {
		actual = append(actual, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}

func TestTaskArtifactIDDoesNotDependOnSuffix(t *testing.T) {
	for _, suffix := range []string{"", ".wav", ".mp3", ".anything"} {
		taskID, ok := taskArtifactID("task_public" + suffix)
		assert.True(t, ok)
		assert.Equal(t, "task_public", taskID)
	}
	for _, artifact := range []string{"image-request.png", "opaque-id"} {
		taskID, ok := taskArtifactID(artifact)
		assert.True(t, ok)
		assert.Equal(t, strings.TrimSuffix(artifact, ".png"), taskID)
	}
}
