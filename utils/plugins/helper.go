package plugins

import (
	"strings"
)

func HasTag(tags []string, target string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, target) {
			return true
		}
	}

	return false
}
