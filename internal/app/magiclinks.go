package app

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const maxMagicLinksPerRule = 10

type MagicLinkRule struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Value    string `json:"value"`
	Template string `json:"template"`
}

type MagicLink struct {
	Label string
	URL   string
}

var magicLinkVarPattern = regexp.MustCompile(`\$(?:\{([^}]+)\}|([A-Za-z0-9_]+))`)

func magicLinkVarName(m []string) string {
	if m[1] != "" {
		return strings.TrimSpace(m[1])
	}
	return m[2]
}

// magic links must start with http(s)
func validMagicLinkTemplate(tmpl string) bool {
	lower := strings.ToLower(tmpl)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func tagValuesFold(tags map[string][]string, name string) []string {
	var values []string
	for cat, vals := range tags {
		if strings.EqualFold(cat, name) {
			values = append(values, vals...)
		}
	}
	sort.Strings(values)
	return values
}

func ruleMatches(rule MagicLinkRule, tags map[string][]string) bool {
	accepted := expandTagWithAliases(rule.Category, rule.Value)
	for _, have := range tagValuesFold(tags, rule.Category) {
		for _, want := range accepted {
			if strings.EqualFold(have, want) {
				return true
			}
		}
	}
	return false
}

// Skip when nothing matches, build multiple with multiple matches
func buildMagicLinks(rules []MagicLinkRule, tags map[string][]string) []MagicLink {
	var links []MagicLink
	seen := make(map[string]bool)

	for _, rule := range rules {
		if !ruleMatches(rule, tags) {
			continue
		}

		// Collect each distinct variable used by the template, in order.
		var varNames []string
		varValues := make(map[string][]string)
		missing := false
		for _, m := range magicLinkVarPattern.FindAllStringSubmatch(rule.Template, -1) {
			name := magicLinkVarName(m)
			key := strings.ToLower(name)
			if _, done := varValues[key]; done {
				continue
			}
			vals := tagValuesFold(tags, name)
			if len(vals) == 0 {
				missing = true
				break
			}
			varNames = append(varNames, key)
			varValues[key] = vals
		}
		if missing {
			continue
		}

		// Expand the product of variable values.
		combos := []map[string]string{{}}
		for _, key := range varNames {
			var next []map[string]string
			for _, combo := range combos {
				for _, v := range varValues[key] {
					c := make(map[string]string, len(combo)+1)
					for k, val := range combo {
						c[k] = val
					}
					c[key] = v
					next = append(next, c)
					if len(next) >= maxMagicLinksPerRule {
						break
					}
				}
				if len(next) >= maxMagicLinksPerRule {
					break
				}
			}
			combos = next
		}

		for _, combo := range combos {
			built := magicLinkVarPattern.ReplaceAllStringFunc(rule.Template, func(s string) string {
				m := magicLinkVarPattern.FindStringSubmatch(s)
				return url.PathEscape(combo[strings.ToLower(magicLinkVarName(m))])
			})
			if seen[built] {
				continue
			}
			seen[built] = true
			links = append(links, MagicLink{Label: rule.Name, URL: built})
		}
	}

	return links
}
