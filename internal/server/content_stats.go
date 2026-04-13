package server

import (
	"strings"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type contentStats struct {
	WordCount  int `json:"wordCount"`
	LineCount  int `json:"lineCount"`
	TokenCount int `json:"tokenCount"`
}

var (
	cl100kEncoderOnce sync.Once
	cl100kEncoder     *tiktoken.Tiktoken
	cl100kEncoderErr  error
)

func buildContentStats(content string) contentStats {
	return contentStats{
		WordCount:  countWords(content),
		LineCount:  countLines(content),
		TokenCount: countTokens(content),
	}
}

func countWords(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 0
	}
	return len(strings.Fields(trimmed))
}

func countLines(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 0
	}
	normalized := strings.ReplaceAll(trimmed, "\r\n", "\n")
	return len(strings.Split(normalized, "\n"))
}

func countTokens(content string) int {
	encoder, err := getCL100KEncoder()
	if err != nil {
		return 0
	}
	return len(encoder.Encode(content, nil, nil))
}

func getCL100KEncoder() (*tiktoken.Tiktoken, error) {
	cl100kEncoderOnce.Do(func() {
		cl100kEncoder, cl100kEncoderErr = tiktoken.GetEncoding("cl100k_base")
	})
	return cl100kEncoder, cl100kEncoderErr
}
