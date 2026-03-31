package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonExtract 通过点分隔路径从 JSON 字节中提取值（如 "data.token"）。
func jsonExtract(body []byte, path string) interface{} {
	var m interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil
	}
	parts := strings.Split(path, ".")
	cur := m
	for _, p := range parts {
		switch v := cur.(type) {
		case map[string]interface{}:
			cur = v[p]
		default:
			return nil
		}
	}
	return cur
}

// contains 判断 body 字节切片是否包含字符串 s。
func contains(body []byte, s string) bool {
	return bytes.Contains(body, []byte(s))
}

// toString 将任意值转为字符串用于断言比较。
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// assertReason 生成断言失败原因描述。
func assertReason(field string, expected, got interface{}) string {
	return fmt.Sprintf("assert %s: expected=%v got=%v", field, expected, got)
}

// assertReasonIn 生成 in 断言失败原因描述。
func assertReasonIn(field string, expected []int, got interface{}) string {
	return fmt.Sprintf("assert %s: expected one of %v got=%v", field, expected, got)
}
