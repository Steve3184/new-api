package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareMappedPassThroughBodyRewritesJSONModel(t *testing.T) {
	payload := []byte(`{"model":"auto/terra","messages":[],"future":{"enabled":true}}`)
	storage, err := common.CreateBodyStorage(payload)
	require.NoError(t, err)
	defer storage.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json; charset=utf-8")
	info := &relaycommon.RelayInfo{
		OriginModelName: "auto/terra",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "openai/gpt-5.6-terra",
		},
	}

	body, closer, err := prepareMappedPassThroughBody(c, storage, info, false)
	require.NoError(t, err)
	require.NotNil(t, closer)
	defer closer.Close()

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "openai/gpt-5.6-terra", gjson.GetBytes(got, "model").String())
	require.True(t, gjson.GetBytes(got, "future.enabled").Bool())
}

func TestPrepareMappedPassThroughBodyPreservesNonJSONBody(t *testing.T) {
	payload := []byte("--boundary\r\nmodel=auto/terra\r\n--boundary--\r\n")
	storage, err := common.CreateBodyStorage(payload)
	require.NoError(t, err)
	defer storage.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	info := &relaycommon.RelayInfo{
		OriginModelName: "auto/terra",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "openai/gpt-5.6-terra",
		},
	}

	body, closer, err := prepareMappedPassThroughBody(c, storage, info, false)
	require.NoError(t, err)
	require.Nil(t, closer)

	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}
