package common

import rootcommon "github.com/QuantumNous/new-api/common"

const claudeCacheControlField = "cache_control"

// ApplyClaudeStablePrefixCacheControl removes caller-provided cache controls
// and places ephemeral breakpoints on the stable system/tools prefix and the
// last user message.
func ApplyClaudeStablePrefixCacheControl(jsonData []byte) ([]byte, error) {
	var body map[string]any
	if err := rootcommon.Unmarshal(jsonData, &body); err != nil {
		return nil, err
	}
	if body == nil {
		return jsonData, nil
	}

	delete(body, claudeCacheControlField)
	clearClaudeSystemCacheControl(body["system"])
	clearClaudeToolCacheControl(body["tools"])
	clearClaudeMessageCacheControl(body["messages"])

	// Tools follow the system prompt in Claude's stable request prefix, so a
	// breakpoint on the final tool caches both sections together.
	if !setClaudeCacheControlOnLastObject(body["tools"]) {
		setClaudeCacheControlOnSystem(body)
	}
	setClaudeCacheControlOnLastUserMessage(body["messages"])
	return rootcommon.Marshal(body)
}

func clearClaudeSystemCacheControl(system any) {
	clearClaudeCacheControlFromContent(system)
}

func clearClaudeToolCacheControl(tools any) {
	if toolList, ok := tools.([]any); ok {
		for _, tool := range toolList {
			if toolMap, ok := tool.(map[string]any); ok {
				delete(toolMap, claudeCacheControlField)
			}
		}
	}
}

func clearClaudeMessageCacheControl(messages any) {
	messageList, ok := messages.([]any)
	if !ok {
		return
	}
	for _, message := range messageList {
		messageMap, ok := message.(map[string]any)
		if !ok {
			continue
		}
		delete(messageMap, claudeCacheControlField)
		clearClaudeCacheControlFromContent(messageMap["content"])
	}
}

func clearClaudeCacheControlFromContent(content any) {
	switch value := content.(type) {
	case map[string]any:
		clearClaudeContentBlockCacheControl(value)
	case []any:
		for _, block := range value {
			if blockMap, ok := block.(map[string]any); ok {
				clearClaudeContentBlockCacheControl(blockMap)
			}
		}
	}
}

func clearClaudeContentBlockCacheControl(block map[string]any) {
	delete(block, claudeCacheControlField)
	if block["type"] == "tool_result" {
		clearClaudeCacheControlFromContent(block["content"])
	}
}

func setClaudeCacheControlOnLastUserMessage(messages any) bool {
	messageList, ok := messages.([]any)
	if !ok {
		return false
	}
	for index := len(messageList) - 1; index >= 0; index-- {
		message, ok := messageList[index].(map[string]any)
		if !ok || message["role"] != "user" {
			continue
		}
		if setClaudeCacheControlOnMessageContent(message) {
			return true
		}
	}
	return false
}

func setClaudeCacheControlOnMessageContent(message map[string]any) bool {
	content, exists := message["content"]
	if !exists {
		return false
	}

	switch value := content.(type) {
	case string:
		if value == "" {
			return false
		}
		message["content"] = []any{map[string]any{
			"type":                  "text",
			"text":                  value,
			claudeCacheControlField: map[string]any{"type": "ephemeral"},
		}}
		return true
	case map[string]any:
		setClaudeCacheControlOnObject(value)
		return true
	default:
		return setClaudeCacheControlOnLastObject(value)
	}
}

func setClaudeCacheControlOnSystem(body map[string]any) bool {
	system, exists := body["system"]
	if !exists {
		return false
	}

	if systemText, ok := system.(string); ok {
		if systemText == "" {
			return false
		}
		body["system"] = []any{map[string]any{
			"type":                  "text",
			"text":                  systemText,
			claudeCacheControlField: map[string]any{"type": "ephemeral"},
		}}
		return true
	}
	return setClaudeCacheControlOnLastObject(system)
}

func setClaudeCacheControlOnLastObject(value any) bool {
	objects, ok := value.([]any)
	if !ok {
		return false
	}
	for index := len(objects) - 1; index >= 0; index-- {
		object, ok := objects[index].(map[string]any)
		if !ok {
			continue
		}
		setClaudeCacheControlOnObject(object)
		return true
	}
	return false
}

func setClaudeCacheControlOnObject(object map[string]any) {
	object[claudeCacheControlField] = map[string]any{"type": "ephemeral"}
}
