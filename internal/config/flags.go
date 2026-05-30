package config

import "strings"

type FlagRule struct {
	exact  string
	prefix string
}

func ParseAllowlist(raw []string) []FlagRule {
	rules := make([]FlagRule, 0, len(raw))
	for _, r := range raw {
		if strings.HasSuffix(r, "*") {
			rules = append(rules, FlagRule{prefix: strings.TrimSuffix(r, "*")})
		} else {
			rules = append(rules, FlagRule{exact: r})
		}
	}
	return rules
}

func Allows(rules []FlagRule, flag string) bool {
	for _, r := range rules {
		if r.exact != "" && r.exact == flag {
			return true
		}
		if r.prefix != "" && strings.HasPrefix(flag, r.prefix) && len(flag) > len(r.prefix) {
			return true
		}
	}
	return false
}
