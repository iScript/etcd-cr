package types

import (
	"strings"
)

// URLsMap 是一个name和URL的映射
type URLsMap map[string]URLs

func NewURLsMap(s string) (URLsMap, error) {
	m := parse(s) // test=http://localhost:2380  => map[test:[http://localhost:2380]]
	cl := URLsMap{}
	for name, urls := range m {
		us, err := NewURLs(urls)
		if err != nil {
			return nil, err
		}
		cl[name] = us
	}
	return cl, nil
}

func parse(s string) map[string][]string {
	m := make(map[string][]string)
	for s != "" {
		key := s
		if i := strings.IndexAny(key, ","); i >= 0 {
			key, s = key[:i], key[i+1:]
		} else {
			s = ""
		}
		if key == "" {
			continue
		}
		value := ""
		if i := strings.Index(key, "="); i >= 0 {
			key, value = key[:i], key[i+1:]
		}
		m[key] = append(m[key], value)
	}
	return m
}
