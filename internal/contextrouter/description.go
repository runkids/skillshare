package contextrouter

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

var (
	descriptionLineRe = regexp.MustCompile(`^description:[ \t]+(.*)$`)
	singleQuotedRe    = regexp.MustCompile(`^'((?:[^']|'')*)'`)
	trailingCommentRe = regexp.MustCompile(`^(?:[ \t]+(?:#.*)?)?$`)
	plainCommentRe    = regexp.MustCompile(`[ \t]+#`)
	plainIndicatorRe  = regexp.MustCompile(`^[-?:](?:[ \t]|$)`)
	plainMappingRe    = regexp.MustCompile(`:(?:[ \t]|$)`)
)

// jsonStringEnd is the end of the JSON string that starts value, or -1.
func jsonStringEnd(value string) int {
	for i := 1; i < len(value); i++ {
		switch value[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

// pageDescription mirrors page_description: the single-line `description:` of
// a page's closed frontmatter envelope, or "" when it has none the router
// reads (a block or folded value, a flow collection, an unterminated quote).
// A topic that leaves its description out shows its first source's.
func pageDescription(lines []string) string {
	end := frontmatterEnd(lines)
	for index := 1; index < end-1; index++ {
		match := descriptionLineRe.FindStringSubmatch(lines[index])
		if match == nil {
			continue
		}
		for _, following := range lines[index+1 : end-1] {
			if isBlank(following) || strings.HasPrefix(pyTrimLeft(following), "#") {
				continue
			}
			if strings.HasPrefix(following, " ") || strings.HasPrefix(following, "\t") {
				return "" // the value continues on indented lines
			}
			break
		}
		value := strings.Trim(match[1], " \t\r")
		var result string
		switch {
		case strings.HasPrefix(value, `"`):
			stop := jsonStringEnd(value)
			if stop < 0 || json.Unmarshal([]byte(value[:stop]), &result) != nil || !trailingCommentRe.MatchString(value[stop:]) {
				return ""
			}
		case strings.HasPrefix(value, "'"):
			quoted := singleQuotedRe.FindStringSubmatch(value)
			if quoted == nil || !trailingCommentRe.MatchString(value[len(quoted[0]):]) {
				return ""
			}
			result = strings.ReplaceAll(quoted[1], "''", "'")
		default:
			result = strings.Trim(plainCommentRe.Split(value, 2)[0], " \t")
			lower := strings.ToLower(result)
			if result == "" || strings.ContainsRune("[]{},#&*!|>%@`", rune(result[0])) ||
				plainIndicatorRe.MatchString(result) || plainMappingRe.MatchString(result) ||
				lower == "~" || lower == "null" {
				return ""
			}
		}
		for _, r := range result {
			if r != '\t' && (unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) || r == unicode.ReplacementChar) {
				return ""
			}
		}
		return result
	}
	return ""
}
