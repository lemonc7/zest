package engx

import (
	"regexp"
)

// func BindPathParams(r *http.Request, dst any) error {
// 	names := getPathParamsNames(r.Pattern)

// 	params := map[string][]string{}
// 	for _, name := range names {
// 		value := r.PathValue(name)
// 		params[name] = []string{value}
// 	}

// }

func getPathParamsNames(pattern string) []string {
	// 匹配{param}
	re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
	matches := re.FindAllStringSubmatch(pattern, -1)
	var params []string
	for _, match := range matches {
		params = append(params, match[1])
	}
	return params
}
