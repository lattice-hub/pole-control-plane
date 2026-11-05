/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package types

import (
	"encoding/json"
	"io"
	"strings"
)

const redactedOperationDetail = `{"redacted":true}`

// SanitizeOperationDetail 清理账户审计中的凭据，供生成、持久化及历史查询边界复用。
// 账户详情必须是 JSON；无法安全解析时返回固定标记，绝不回退到原文。
// 其他资源原样保留；嵌套对象、数组及 metadata 中的 JSON 字符串遵循同一规则。
func SanitizeOperationDetail(resource Resource, detail string) string {
	if resource != RUser && resource != RUserGroup && resource != RUserGroupRelation {
		return detail
	}
	if detail == "" {
		return ""
	}
	value, err := decodeOperationDetail(detail)
	if err != nil {
		return redactedOperationDetail
	}
	switch value.(type) {
	case map[string]any, []any:
	case string:
		if !looksLikeOperationJSON(value.(string)) {
			return redactedOperationDetail
		}
	default:
		return redactedOperationDetail
	}
	encoded, err := json.Marshal(sanitizeOperationValue(value, 0))
	if err != nil {
		return redactedOperationDetail
	}
	return string(encoded)
}

func decodeOperationDetail(detail string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(detail))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	return value, nil
}

func sanitizeOperationValue(value any, depth int) any {
	// 限制递归和重复编码层数；超出时隐藏该子树，不把未经检查的内容交给 sink。
	if depth >= 64 {
		return "[REDACTED]"
	}
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			if isOperationCredential(key) {
				delete(current, key)
				continue
			}
			current[key] = sanitizeOperationValue(child, depth+1)
		}
	case []any:
		for index, child := range current {
			current[index] = sanitizeOperationValue(child, depth+1)
		}
	case string:
		if !looksLikeOperationJSON(current) {
			return current
		}
		child, err := decodeOperationDetail(current)
		if err != nil {
			return "[REDACTED]"
		}
		encoded, err := json.Marshal(sanitizeOperationValue(child, depth+1))
		if err != nil {
			return "[REDACTED]"
		}
		return string(encoded)
	}
	return value
}

func looksLikeOperationJSON(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "{") ||
		strings.HasPrefix(value, "[") || strings.HasPrefix(value, `"`)
}

func isOperationCredential(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	switch key {
	case "password", "oldpassword", "newpassword", "passwordhash", "authtoken", "token":
		return true
	default:
		return false
	}
}
