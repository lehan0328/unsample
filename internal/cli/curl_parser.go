package cli

import (
	"fmt"
	"net/http"
	"strings"
)

// ParseCurl parses a curl command string into an HTTP request.
// It supports the most common curl flags: -X (method), -H (headers),
// -d/--data (body), and the URL.
//
// Example input: curl -X POST -H 'Content-Type: application/json' -d '{"key":"val"}' https://api.example.com/endpoint
func ParseCurl(curlCmd string) (*http.Request, error) {
	args, err := tokenize(curlCmd)
	if err != nil {
		return nil, fmt.Errorf("tokenizing curl command: %w", err)
	}

	method := "GET"
	var url string
	var headers [][2]string
	var body string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "curl":
			// Skip the "curl" command itself.
			continue

		case arg == "-X" || arg == "--request":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			i++
			method = strings.ToUpper(args[i])

		case arg == "-H" || arg == "--header":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			i++
			key, val, ok := strings.Cut(args[i], ":")
			if !ok {
				return nil, fmt.Errorf("invalid header format %q (expected Key: Value)", args[i])
			}
			headers = append(headers, [2]string{strings.TrimSpace(key), strings.TrimSpace(val)})

		case arg == "-d" || arg == "--data" || arg == "--data-raw":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			i++
			body = args[i]
			if method == "GET" {
				method = "POST" // curl defaults to POST when -d is used
			}

		case strings.HasPrefix(arg, "-"):
			// Skip unknown flags (like -s, -v, -k, etc.).
			// If the flag takes a value, peek ahead.
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && !isURL(args[i+1]) {
				i++ // skip the value too
			}

		default:
			// Positional argument — treat as URL.
			if url == "" {
				url = arg
			}
		}
	}

	if url == "" {
		return nil, fmt.Errorf("no URL found in curl command")
	}

	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	var req *http.Request
	if bodyReader != nil {
		req, err = http.NewRequest(method, url, bodyReader)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}

	return req, nil
}

// isURL is a simple heuristic to check if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// tokenize splits a curl command string into tokens, respecting single and
// double quotes. This handles cases like: -H 'Content-Type: application/json'
func tokenize(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\\' && inDouble && i+1 < len(input):
			i++
			current.WriteByte(input[i])
		case (ch == ' ' || ch == '\t' || ch == '\n') && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in curl command")
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}
