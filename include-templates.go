package main

import (
	"fmt"
	"html/template"
	"strings"
)

// InitTemplates loads and parses all HTML templates with helper functions
func InitTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"hasAnySuffix": func(s string, suffixes ...string) bool {
			for _, suf := range suffixes {
				if strings.HasSuffix(strings.ToLower(s), suf) {
					return true
				}
			}
			return false
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of args")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	}).ParseGlob("templates/*.html")
}
