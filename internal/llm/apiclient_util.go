package llm

import (
	"bufio"
	"encoding/json"
	"io"
)

// serverSentEventScanner reads Server-Sent Events from a stream.
type serverSentEventScanner struct {
	scanner *bufio.Scanner
}

// newServerSentEventScanner creates a new SSE scanner.
func newServerSentEventScanner(r io.Reader) *serverSentEventScanner {
	return &serverSentEventScanner{
		scanner: bufio.NewScanner(r),
	}
}

// Scan reads the next line of data.
func (s *serverSentEventScanner) Scan() bool {
	return s.scanner.Scan()
}

// Text returns the last scanned line.
func (s *serverSentEventScanner) Text() string {
	return s.scanner.Text()
}

// parseJSONSchema converts a JSON schema string to a map.
func parseJSONSchema(schemaStr string) map[string]interface{} {
	if schemaStr == "" {
		return nil
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		// If parsing fails, return nil - the API will handle the error
		return nil
	}

	return schema
}
