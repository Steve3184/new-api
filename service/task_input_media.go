package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	MaxTaskInputMediaBytes      = 20 << 20
	MaxTaskInputMediaTotalBytes = 64 << 20
	maxTaskInputMediaCapacity   = 256 << 20
	taskInputMediaTTL           = time.Hour
)

var ErrTaskInputMedia = errors.New("invalid or unavailable task input media")

type taskInputMedia struct {
	data     []byte
	mimeType string
	expires  time.Time
	key      string
	ownerID  int
}

// Temporary references live on the accepting instance and disappear on restart.
var taskInputs = struct {
	sync.Mutex
	items map[string]taskInputMedia
	bytes int
	once  sync.Once
}{items: make(map[string]taskInputMedia)}

// DecodeTaskInputMedia checks media signatures instead of trusting the declared
// MIME type. SVG and arbitrary files are deliberately unsupported.
func DecodeTaskInputMedia(value, kind string) ([]byte, string, error) {
	if kind != "image" && kind != "audio" && kind != "video" {
		return nil, "", ErrTaskInputMedia
	}
	declared := ""
	if strings.HasPrefix(value, "data:") {
		header, payload, ok := strings.Cut(value, ";base64,")
		if !ok {
			return nil, "", ErrTaskInputMedia
		}
		declared, value = strings.TrimPrefix(header, "data:"), payload
	}
	if len(value) == 0 || len(value) > base64.StdEncoding.EncodedLen(MaxTaskInputMediaBytes) {
		return nil, "", ErrTaskInputMedia
	}
	data, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(data) == 0 || len(data) > MaxTaskInputMediaBytes {
		return nil, "", ErrTaskInputMedia
	}
	detected, _, _ := strings.Cut(http.DetectContentType(data), ";")
	if bytes.HasPrefix(data, []byte("fLaC")) {
		detected = "audio/flac"
	}
	if len(data) >= 7 && data[0] == 0xff && data[1]&0xf6 == 0xf0 {
		detected = "audio/aac"
	}
	switch detected {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	case "audio/wave", "audio/wav", "audio/x-wav":
		detected = "audio/wav"
	case "audio/mpeg", "audio/ogg", "audio/flac", "audio/aac":
	case "application/ogg":
		detected = "audio/ogg"
	case "video/mp4":
		if kind == "audio" && declared == "audio/mp4" {
			detected = "audio/mp4"
		} else if kind != "video" {
			return nil, "", ErrTaskInputMedia
		}
	case "video/webm":
		if kind == "audio" && declared == "audio/webm" {
			detected = "audio/webm"
		} else if kind != "video" {
			return nil, "", ErrTaskInputMedia
		}
	default:
		return nil, "", ErrTaskInputMedia
	}
	if !strings.HasPrefix(detected, kind+"/") {
		return nil, "", ErrTaskInputMedia
	}
	switch declared {
	case "audio/wave", "audio/x-wav":
		declared = "audio/wav"
	case "audio/x-flac":
		declared = "audio/flac"
	case "audio/mp3":
		declared = "audio/mpeg"
	}
	if declared != "" && declared != detected {
		return nil, "", ErrTaskInputMedia
	}
	return data, detected, nil
}

// StoreTaskInputMediaBatch publishes a complete batch atomically. Each HMAC
// binds a random object ID to its expiry without exposing signing keys to JS.
func StoreTaskInputMediaBatch(ownerID int, data [][]byte, mimeTypes []string) ([]string, error) {
	if ownerID <= 0 || len(data) != len(mimeTypes) || len(data) == 0 || len(data) > 16 {
		return nil, ErrTaskInputMedia
	}
	base := strings.TrimSpace(system_setting.TaskPublicAddress)
	if base == "" {
		base = strings.TrimSpace(system_setting.ServerAddress)
	}
	if err := ValidateTaskArtifactBaseURL(base); err != nil {
		return nil, err
	}
	expires := time.Now().Add(taskInputMediaTTL).Truncate(time.Second)
	ids, urls := make([]string, len(data)), make([]string, len(data))
	keys := make([]string, len(data))
	key := "input-" + strconv.FormatInt(expires.Unix(), 10)
	total := 0
	for i := range data {
		if len(data[i]) == 0 || len(data[i]) > MaxTaskInputMediaBytes {
			return nil, ErrTaskInputMedia
		}
		extension := ""
		switch mimeTypes[i] {
		case "image/png":
			extension = "png"
		case "image/jpeg":
			extension = "jpg"
		case "image/gif":
			extension = "gif"
		case "image/webp":
			extension = "webp"
		case "audio/wav":
			extension = "wav"
		case "audio/mpeg":
			extension = "mp3"
		case "audio/ogg":
			extension = "ogg"
		case "audio/flac":
			extension = "flac"
		case "audio/aac":
			extension = "aac"
		case "audio/mp4":
			extension = "m4a"
		case "audio/webm":
			extension = "webm"
		case "video/mp4":
			extension = "mp4"
		case "video/webm":
			extension = "webm"
		default:
			return nil, ErrTaskInputMedia
		}
		keys[i] = key + "." + extension
		total += len(data[i])
		if total > MaxTaskInputMediaTotalBytes {
			return nil, ErrTaskInputMedia
		}
		ids[i] = uuid.NewString()
		access, err := IssueTaskArtifactAccess(ids[i], keys[i])
		if err != nil {
			return nil, err
		}
		address, err := url.Parse(base)
		if err != nil {
			return nil, err
		}
		address.Path = strings.TrimRight(address.Path, "/") + "/v1/task-inputs/" + ids[i] + "/" + keys[i]
		address.RawPath = ""
		query := address.Query()
		query.Set(TaskArtifactAccessQueryParameter, access)
		address.RawQuery = query.Encode()
		urls[i] = address.String()
	}
	taskInputs.Lock()
	defer taskInputs.Unlock()
	ownerBytes, ownerCount := 0, 0
	for id, item := range taskInputs.items {
		if !time.Now().Before(item.expires) {
			delete(taskInputs.items, id)
			taskInputs.bytes -= len(item.data)
			continue
		}
		if item.ownerID == ownerID {
			ownerBytes += len(item.data)
			ownerCount++
		}
	}
	if ownerBytes+total > MaxTaskInputMediaTotalBytes || ownerCount+len(data) > 128 {
		return nil, fmt.Errorf("temporary input media user capacity exceeded")
	}
	if taskInputs.bytes+total > maxTaskInputMediaCapacity || len(taskInputs.items)+len(data) > 1024 {
		return nil, fmt.Errorf("temporary input media capacity exceeded")
	}
	for i, id := range ids {
		taskInputs.items[id] = taskInputMedia{data: bytes.Clone(data[i]), mimeType: mimeTypes[i], expires: expires, key: keys[i], ownerID: ownerID}
	}
	taskInputs.bytes += total
	taskInputs.once.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				taskInputs.Lock()
				for id, item := range taskInputs.items {
					if !time.Now().Before(item.expires) {
						delete(taskInputs.items, id)
						taskInputs.bytes -= len(item.data)
					}
				}
				taskInputs.Unlock()
			}
		}()
	})
	return urls, nil
}

// ServeTaskInputMedia requires the caller to have verified the signed capability.
func ServeTaskInputMedia(c *gin.Context) {
	taskInputs.Lock()
	item, ok := taskInputs.items[c.Param("key")]
	taskInputs.Unlock()
	if !ok || item.key != c.Param("artifact_key") || !time.Now().Before(item.expires) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", item.mimeType)
	c.Header("Content-Disposition", "attachment")
	http.ServeContent(c.Writer, c.Request, "reference", time.Time{}, bytes.NewReader(item.data))
}
