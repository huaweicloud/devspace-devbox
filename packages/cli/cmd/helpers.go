package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------- 指针工具（oapi-codegen 生成的类型大量使用指针）----------

func pInt(i int32) *int        { v := int(i); return &v }
func pInt32(i int32) *int32    { return &i }
func pString(s string) *string { return &s }

// pStringMap 在 map 为空时返回 nil，确保对应字段不出现在 JSON body 中。
func pStringMap(m map[string]string) *map[string]string {
	if len(m) == 0 {
		return nil
	}
	return &m
}

// optString 将空字符串转为 nil 指针。
func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---------- 参数解析 ----------

// parseKV 将重复的 key=value 参数解析为 map。
func parseKV(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("expected key=value, got %q", p)
		}
		out[k] = v
	}
	return out, nil
}

// parseMetadata 解析 --metadata 参数。
func parseMetadata(pairs []string) (*map[string]string, error) {
	m, err := parseKV(pairs)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return &m, nil
}

// ---------- 时间与展示 ----------

// formatTime 将时间转换为当地时间并格式化为 "2006-01-02 15:04:05"。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// formatTimePtr 渲染可空时间字段（如 Sandbox.EndAt），nil 时显示 "-"。
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return formatTime(*t)
}

// joinList 用 ", " 连接字符串列表。
func joinList(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}

// formatMetadata 渲染沙箱 metadata，格式与 e2b 一致（JSON），nil 或空时为 "{}"。
func formatMetadata(m *map[string]string) string {
	if m == nil {
		return "{}"
	}
	return formatJSON(*m)
}

// formatJSON 以 JSON 渲染对象（与 e2b 展示格式一致），失败时为 "{}"。
func formatJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
