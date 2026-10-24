package jsplugin

import (
	"bytes"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

// resolveSignedInputMedia publishes only the selected submit descriptor. Decode
// hooks stay side-effect free and normalized billing facts remain unchanged.
func resolveSignedInputMedia(descriptor *requestDescriptor, ownerID int) error {
	if descriptor.BodyType == "multipart" {
		return nil
	}
	body := descriptor.Body
	if descriptor.BodyText != nil {
		if !bytes.Contains(descriptor.BodyText, []byte("__mediaData")) {
			return nil
		}
		if err := common.Unmarshal(descriptor.BodyText, &body); err != nil {
			return err
		}
	}
	if body == nil {
		return nil
	}
	body = jsonValue(body)
	var data [][]byte
	var mimeTypes []string
	var targets []*string
	type mediaIdentity struct{ value, kind string }
	seen := make(map[mediaIdentity]*string)
	total := 0
	var walk func(any, int) (any, error)
	walk = func(value any, depth int) (any, error) {
		if depth > 32 {
			return nil, fmt.Errorf("input media nesting exceeds limit")
		}
		switch typed := value.(type) {
		case map[string]any:
			if raw, exists := typed["__mediaData"]; exists {
				if len(typed) != 2 || len(data) >= 16 {
					return nil, service.ErrTaskInputMedia
				}
				encoded, ok := raw.(string)
				if !ok {
					return nil, service.ErrTaskInputMedia
				}
				kind, ok := typed["kind"].(string)
				if !ok {
					return nil, service.ErrTaskInputMedia
				}
				identity := mediaIdentity{value: encoded, kind: kind}
				if target, exists := seen[identity]; exists {
					return target, nil
				}
				decoded, mimeType, err := service.DecodeTaskInputMedia(encoded, kind)
				if err != nil {
					return nil, err
				}
				total += len(decoded)
				if total > service.MaxTaskInputMediaTotalBytes {
					return nil, service.ErrTaskInputMedia
				}
				target := new(string)
				seen[identity] = target
				data = append(data, decoded)
				mimeTypes = append(mimeTypes, mimeType)
				targets = append(targets, target)
				return target, nil
			}
			for key, child := range typed {
				replaced, err := walk(child, depth+1)
				if err != nil {
					return nil, err
				}
				typed[key] = replaced
			}
		case []any:
			for i, child := range typed {
				replaced, err := walk(child, depth+1)
				if err != nil {
					return nil, err
				}
				typed[i] = replaced
			}
		}
		return value, nil
	}
	replaced, err := walk(body, 0)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	urls, err := service.StoreTaskInputMediaBatch(ownerID, data, mimeTypes)
	if err != nil {
		return err
	}
	for i := range urls {
		*targets[i] = urls[i]
	}
	descriptor.Body = replaced
	descriptor.BodyText = nil
	return nil
}
