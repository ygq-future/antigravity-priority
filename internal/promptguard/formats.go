package promptguard

import (
	"bytes"
	"encoding/json"
	"strings"
)

func normalizeRequestFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", FormatAuto:
		if strings.EqualFold(strings.TrimSpace(raw), FormatAuto) {
			return FormatAuto
		}
		return ""
	case FormatText:
		return FormatText
	case FormatClaude, "anthropic":
		return FormatClaude
	case FormatOpenAI, "chat":
		return FormatOpenAI
	case FormatResponses, "responses", "codex":
		return FormatResponses
	case FormatGemini, "antigravity":
		return FormatGemini
	default:
		return ""
	}
}

// DetectFormat sniffs the protocol shape of a JSON request payload.
func DetectFormat(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return ""
	}
	return detectObjectFormat(obj)
}

func detectObjectFormat(obj map[string]json.RawMessage) string {
	if _, ok := obj["systemInstruction"]; ok {
		return FormatGemini
	}
	if _, ok := obj["system_instruction"]; ok {
		return FormatGemini
	}
	if _, ok := obj["contents"]; ok {
		return FormatGemini
	}
	if reqRaw, ok := obj["request"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(reqRaw, &inner); err == nil && inner != nil {
			if _, okSys := inner["systemInstruction"]; okSys {
				return FormatGemini
			}
			if _, okSysSnake := inner["system_instruction"]; okSysSnake {
				return FormatGemini
			}
			if _, okContents := inner["contents"]; okContents {
				return FormatGemini
			}
		}
	}
	if _, ok := obj["instructions"]; ok {
		return FormatResponses
	}
	if _, ok := obj["input"]; ok {
		return FormatResponses
	}
	if _, ok := obj["system"]; ok {
		return FormatClaude
	}
	if _, ok := obj["messages"]; ok {
		return FormatOpenAI
	}
	return ""
}

func rewriteJSONPayload(format string, body []byte, systemRules, allRules []*compiledRule, hits *hitCounter) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return body, false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &root); err != nil || root == nil {
		return body, false
	}

	var changed bool
	switch format {
	case FormatClaude:
		changed = applyAnthropicObject(root, systemRules, allRules, hits)
	case FormatOpenAI:
		changed = applyChatObject(root, systemRules, allRules, hits)
	case FormatResponses:
		changed = applyResponsesObject(root, systemRules, allRules, hits)
	case FormatGemini:
		changed = applyGeminiObject(root, systemRules, allRules, hits)
	default:
		return body, false
	}
	if !changed {
		return body, false
	}
	out, err := json.Marshal(root)
	if err != nil {
		return body, false
	}
	return out, true
}

func applyAnthropicObject(root map[string]json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) bool {
	changed := false
	if rawSys, ok := root["system"]; ok {
		if updated, okChanged := rewriteContentRaw(rawSys, systemRules, hits); okChanged {
			root["system"] = updated
			changed = true
		}
	}
	if rawMsgs, ok := root["messages"]; ok {
		if updated, okChanged := rewriteMessagesRaw(rawMsgs, systemRules, allRules, hits); okChanged {
			root["messages"] = updated
			changed = true
		}
	}
	return changed
}

func applyChatObject(root map[string]json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) bool {
	rawMsgs, ok := root["messages"]
	if !ok {
		return false
	}
	if updated, okChanged := rewriteMessagesRaw(rawMsgs, systemRules, allRules, hits); okChanged {
		root["messages"] = updated
		return true
	}
	return false
}

func applyResponsesObject(root map[string]json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) bool {
	changed := false
	if rawInst, ok := root["instructions"]; ok {
		if updated, okChanged := rewriteStringRaw(rawInst, systemRules, hits); okChanged {
			root["instructions"] = updated
			changed = true
		}
	}
	if rawInput, ok := root["input"]; ok && len(systemRules) > 0 {
		if updated, okChanged := rewriteResponsesInputRaw(rawInput, systemRules, allRules, hits); okChanged {
			root["input"] = updated
			changed = true
		}
	}
	return changed
}

func applyGeminiObject(root map[string]json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) bool {
	changed := false
	for _, key := range []string{"systemInstruction", "system_instruction"} {
		if rawSys, ok := root[key]; ok {
			if updated, okChanged := rewritePartsContainerRaw(rawSys, systemRules, hits); okChanged {
				root[key] = updated
				changed = true
			}
		}
	}
	if rawContents, ok := root["contents"]; ok {
		var items []json.RawMessage
		if err := json.Unmarshal(rawContents, &items); err == nil && len(items) > 0 {
			contentsChanged := false
			for i, itemRaw := range items {
				rules := geminiItemRules(itemRaw, systemRules, allRules)
				if len(rules) == 0 {
					continue
				}
				if updated, okChanged := rewritePartsContainerRaw(itemRaw, rules, hits); okChanged {
					items[i] = updated
					contentsChanged = true
				}
			}
			if contentsChanged {
				if encoded, err := json.Marshal(items); err == nil {
					root["contents"] = encoded
					changed = true
				}
			}
		}
	}
	if rawReq, ok := root["request"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(rawReq, &inner); err == nil && inner != nil {
			if applyGeminiObject(inner, systemRules, allRules, hits) {
				if encoded, err := json.Marshal(inner); err == nil {
					root["request"] = encoded
					changed = true
				}
			}
		}
	}
	return changed
}

func geminiItemRules(itemRaw json.RawMessage, systemRules, allRules []*compiledRule) []*compiledRule {
	if isSystemRoleRaw(itemRaw) {
		return systemRules
	}
	return allRules
}

func rewriteResponsesInputRaw(raw json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw, false
	}
	switch trimmed[0] {
	case '"':
		return rewriteStringRaw(raw, allRules, hits)
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return raw, false
		}
		anyChanged := false
		for i, itemRaw := range items {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(itemRaw, &obj); err != nil || obj == nil {
				continue
			}
			rules := allRules
			if isSystemRoleObj(obj) {
				rules = systemRules
			}
			if len(rules) == 0 {
				continue
			}
			if contentRaw, ok := obj["content"]; ok {
				if updated, okChanged := rewriteContentRaw(contentRaw, rules, hits); okChanged {
					obj["content"] = updated
					if encoded, err := json.Marshal(obj); err == nil {
						items[i] = encoded
						anyChanged = true
					}
				}
			}
		}
		if !anyChanged {
			return raw, false
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			return raw, false
		}
		return encoded, true
	default:
		return raw, false
	}
}

func rewriteMessagesRaw(raw json.RawMessage, systemRules, allRules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	var msgs []json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil || len(msgs) == 0 {
		return raw, false
	}
	anyChanged := false
	for i, msgRaw := range msgs {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &obj); err != nil || obj == nil {
			continue
		}
		rules := allRules
		if isSystemRoleObj(obj) {
			rules = systemRules
		}
		if len(rules) == 0 {
			continue
		}
		contentRaw, ok := obj["content"]
		if !ok {
			continue
		}
		if updated, okChanged := rewriteContentRaw(contentRaw, rules, hits); okChanged {
			obj["content"] = updated
			if encoded, err := json.Marshal(obj); err == nil {
				msgs[i] = encoded
				anyChanged = true
			}
		}
	}
	if !anyChanged {
		return raw, false
	}
	encoded, err := json.Marshal(msgs)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func rewriteContentRaw(raw json.RawMessage, rules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	if len(rules) == 0 {
		return raw, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw, false
	}
	switch trimmed[0] {
	case '"':
		return rewriteStringRaw(trimmed, rules, hits)
	case '[':
		var blocks []json.RawMessage
		if err := json.Unmarshal(trimmed, &blocks); err != nil || len(blocks) == 0 {
			return raw, false
		}
		anyChanged := false
		for i, blockRaw := range blocks {
			if updated, okChanged := rewriteTextFieldInObjectRaw(blockRaw, rules, hits); okChanged {
				blocks[i] = updated
				anyChanged = true
			}
		}
		if !anyChanged {
			return raw, false
		}
		encoded, err := json.Marshal(blocks)
		if err != nil {
			return raw, false
		}
		return encoded, true
	default:
		return raw, false
	}
}

func rewritePartsContainerRaw(raw json.RawMessage, rules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	if len(rules) == 0 {
		return raw, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw, false
	}
	var container map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &container); err != nil || container == nil {
		return raw, false
	}
	partsRaw, ok := container["parts"]
	if !ok {
		return raw, false
	}
	var parts []json.RawMessage
	if err := json.Unmarshal(partsRaw, &parts); err != nil || len(parts) == 0 {
		return raw, false
	}
	anyChanged := false
	for i, partRaw := range parts {
		if updated, okChanged := rewriteTextFieldInObjectRaw(partRaw, rules, hits); okChanged {
			parts[i] = updated
			anyChanged = true
		}
	}
	if !anyChanged {
		return raw, false
	}
	encodedParts, err := json.Marshal(parts)
	if err != nil {
		return raw, false
	}
	container["parts"] = encodedParts
	encodedContainer, err := json.Marshal(container)
	if err != nil {
		return raw, false
	}
	return encodedContainer, true
}

func rewriteTextFieldInObjectRaw(raw json.RawMessage, rules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil || obj == nil {
		return raw, false
	}
	textRaw, ok := obj["text"]
	if !ok {
		return raw, false
	}
	updatedText, changed := rewriteStringRaw(textRaw, rules, hits)
	if !changed {
		return raw, false
	}
	obj["text"] = updatedText
	encoded, err := json.Marshal(obj)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func rewriteStringRaw(raw json.RawMessage, rules []*compiledRule, hits *hitCounter) (json.RawMessage, bool) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return raw, false
	}
	rewritten, changed := rewriteText(text, rules, hits)
	if !changed {
		return raw, false
	}
	encoded, err := json.Marshal(rewritten)
	if err != nil {
		return raw, false
	}
	return encoded, true
}

func isSystemRoleRaw(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var roleObj struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(trimmed, &roleObj); err != nil {
		return false
	}
	return isSystemRoleValue(roleObj.Role)
}

func isSystemRoleObj(obj map[string]json.RawMessage) bool {
	roleRaw, ok := obj["role"]
	if !ok {
		return false
	}
	var role string
	if err := json.Unmarshal(roleRaw, &role); err != nil {
		return false
	}
	return isSystemRoleValue(role)
}

func isSystemRoleValue(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "developer":
		return true
	default:
		return false
	}
}
