// Package parse 提供 key=value 形式命令行参数的解析工具。
package parse

import (
	"fmt"
	"strings"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// ParseKV 将重复的 key=value 参数解析为 map；按第一个 "=" 切分，
// value 中可以再包含 "="。
func ParseKV(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf(i18n.T(i18n.Msg.Parse.ExpectedKeyValue), p)
		}
		out[k] = v
	}
	return out, nil
}

// ParseMetadata 解析 --metadata 参数，空时返回 nil。
func ParseMetadata(pairs []string) (*map[string]string, error) {
	m, err := ParseKV(pairs)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return &m, nil
}
