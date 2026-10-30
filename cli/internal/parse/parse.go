// Package parse provides parsing helpers for key=value command line arguments.
package parse

import (
	"fmt"
	"strings"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// ParseKV parses repeated key=value arguments into a map; split on the first "=",
// the value may contain further "=" characters.
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

// ParseMetadata parses the --metadata argument; empty returns nil.
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
