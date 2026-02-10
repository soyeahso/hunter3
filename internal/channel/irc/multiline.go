package irc

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/lrstanley/girc"
)

const (
	capMultiline       = "draft/multiline"
	tagMultilineConcat = "draft/multiline-concat"
	tagBatch           = "batch"
	cmdBATCH           = "BATCH"
)

// multilineCaps holds server-advertised limits for draft/multiline.
type multilineCaps struct {
	maxBytes int // 0 means no limit known
	maxLines int // 0 means no limit known
}

// batchEntry represents a single line within an active multiline batch.
type batchEntry struct {
	text   string
	concat bool
}

// activeBatch tracks an in-progress inbound multiline batch.
type activeBatch struct {
	batchType string
	target    string
	source    *girc.Source
	entries   []batchEntry
}

// batchTracker manages active inbound multiline batches.
type batchTracker struct {
	mu      sync.Mutex
	batches map[string]*activeBatch
}

func newBatchTracker() *batchTracker {
	return &batchTracker{
		batches: make(map[string]*activeBatch),
	}
}

// start opens a new batch. Returns false if the batch type is not draft/multiline.
func (bt *batchTracker) start(id, batchType, target string, source *girc.Source) bool {
	if batchType != capMultiline {
		return false
	}
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.batches[id] = &activeBatch{
		batchType: batchType,
		target:    target,
		source:    source,
	}
	return true
}

// addLine appends a line to an active batch. Returns false if the batch ID is unknown.
func (bt *batchTracker) addLine(batchID, text string, concat bool) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	batch, ok := bt.batches[batchID]
	if !ok {
		return false
	}
	batch.entries = append(batch.entries, batchEntry{text: text, concat: concat})
	return true
}

// end closes a batch and returns its assembled content.
func (bt *batchTracker) end(id string) (target string, source *girc.Source, body string, ok bool) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	batch, exists := bt.batches[id]
	if !exists {
		return "", nil, "", false
	}
	delete(bt.batches, id)
	return batch.target, batch.source, assembleBatch(batch.entries), true
}

// has checks whether the given batch ID is being tracked.
func (bt *batchTracker) has(batchID string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	_, ok := bt.batches[batchID]
	return ok
}

// assembleBatch joins batch entries into a single message body.
// Lines are joined with '\n' unless the entry has the concat flag,
// in which case it is appended directly to the previous line.
func assembleBatch(entries []batchEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, entry := range entries {
		if i > 0 && !entry.concat {
			b.WriteByte('\n')
		}
		b.WriteString(entry.text)
	}
	return b.String()
}

// generateBatchID creates a random batch reference tag.
func generateBatchID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseMultilineCaps extracts max-bytes and max-lines from a CAP LS value string.
// The format is: draft/multiline=max-bytes=4096,max-lines=24
func parseMultilineCaps(capValue string) multilineCaps {
	var caps multilineCaps
	for _, token := range strings.Split(capValue, ",") {
		k, v, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		switch k {
		case "max-bytes":
			caps.maxBytes, _ = strconv.Atoi(v)
		case "max-lines":
			caps.maxLines, _ = strconv.Atoi(v)
		}
	}
	return caps
}

// sendMultiline sends a message body as one or more draft/multiline BATCH blocks.
// It splits on newlines and respects the server's max-lines and max-bytes limits.
// If the message exceeds these limits, multiple batches are sent.
// Lines exceeding the per-line byte limit are split with the concat tag.
func sendMultiline(client *girc.Client, target, body string, caps multilineCaps) {
	// IRC line limit for the message portion. Account for protocol overhead:
	// @batch=<id> PRIVMSG <target> :<text>\r\n
	// Tags, command, target, and framing consume ~60+ bytes; use 400 as safe limit.
	perLineMax := 400

	allLines := strings.Split(body, "\n")
	if len(allLines) == 0 {
		return
	}

	// Determine limits per batch
	maxLinesPerBatch := caps.maxLines
	if maxLinesPerBatch <= 0 {
		maxLinesPerBatch = len(allLines) // no limit
	}

	maxBytesPerBatch := caps.maxBytes
	if maxBytesPerBatch <= 0 {
		maxBytesPerBatch = 0 // no limit
	}

	// Split message into batches
	for len(allLines) > 0 {
		// Determine how many lines fit in this batch
		var batchLines []string
		totalBytes := 0

		for i, line := range allLines {
			// Check line count limit
			if i >= maxLinesPerBatch {
				break
			}

			// Calculate bytes for this line (including newline separator)
			lineBytes := len(line)
			if i > 0 {
				lineBytes++ // account for \n separator
			}

			// Check byte count limit
			if maxBytesPerBatch > 0 && totalBytes+lineBytes > maxBytesPerBatch {
				// If this is the first line and it doesn't fit, truncate it
				if i == 0 {
					remaining := maxBytesPerBatch - totalBytes
					if remaining > 0 {
						batchLines = append(batchLines, line[:remaining])
						totalBytes += remaining
					}
				}
				break
			}

			batchLines = append(batchLines, line)
			totalBytes += lineBytes
		}

		if len(batchLines) == 0 {
			break // nothing more to send
		}

		// Send this batch
		sendSingleBatch(client, target, batchLines, perLineMax)

		// Move to next batch
		allLines = allLines[len(batchLines):]
	}
}

// sendSingleBatch sends a single BATCH block with the given lines.
func sendSingleBatch(client *girc.Client, target string, lines []string, perLineMax int) {
	batchID := generateBatchID()

	// BATCH +<id> draft/multiline <target>
	client.Send(&girc.Event{
		Command: cmdBATCH,
		Params:  []string{"+" + batchID, capMultiline, target},
	})

	for _, line := range lines {
		sendBatchLine(client, batchID, target, line, perLineMax)
	}

	// BATCH -<id>
	client.Send(&girc.Event{
		Command: cmdBATCH,
		Params:  []string{"-" + batchID},
	})
}

// sendBatchLine sends a single line within a batch, splitting with concat tags
// if the line exceeds maxLen bytes.
func sendBatchLine(client *girc.Client, batchID, target, line string, maxLen int) {
	if len(line) <= maxLen {
		client.Send(&girc.Event{
			Command: girc.PRIVMSG,
			Params:  []string{target, line},
			Tags:    girc.Tags{tagBatch: batchID},
		})
		return
	}

	// Split long lines using the concat tag.
	for len(line) > 0 {
		end := maxLen
		if end > len(line) {
			end = len(line)
		}
		chunk := line[:end]
		line = line[end:]

		tags := girc.Tags{tagBatch: batchID}
		if len(line) > 0 {
			// More chunks follow — mark this one with concat so the receiver
			// joins them without inserting a newline.
			tags[tagMultilineConcat] = ""
		}

		client.Send(&girc.Event{
			Command: girc.PRIVMSG,
			Params:  []string{target, chunk},
			Tags:    tags,
		})
	}
}

// parseBatchStart parses a BATCH +<id> event.
// Returns the batch ID, type, target, and whether this is a valid start.
func parseBatchStart(e girc.Event) (id, batchType, target string, ok bool) {
	if len(e.Params) < 2 {
		return "", "", "", false
	}
	ref := e.Params[0]
	if len(ref) < 2 || ref[0] != '+' {
		return "", "", "", false
	}
	id = ref[1:]
	batchType = e.Params[1]
	if len(e.Params) >= 3 {
		target = e.Params[2]
	}
	return id, batchType, target, true
}

// parseBatchEnd parses a BATCH -<id> event.
// Returns the batch ID and whether this is a valid end.
func parseBatchEnd(e girc.Event) (id string, ok bool) {
	if len(e.Params) < 1 {
		return "", false
	}
	ref := e.Params[0]
	if len(ref) < 2 || ref[0] != '-' {
		return "", false
	}
	return ref[1:], true
}

// isBatchEvent checks if an event has a "batch" tag referencing an active batch.
func isBatchEvent(e girc.Event) (batchID string, ok bool) {
	if e.Tags == nil {
		return "", false
	}
	id, ok := e.Tags.Get(tagBatch)
	return id, ok && id != ""
}

// hasConcatTag checks if the event has the draft/multiline-concat tag.
func hasConcatTag(e girc.Event) bool {
	if e.Tags == nil {
		return false
	}
	_, ok := e.Tags[tagMultilineConcat]
	return ok
}

// extractMultilineCapsFromLS parses a CAP LS response string and extracts
// draft/multiline parameters if present.
func extractMultilineCapsFromLS(capLS string) (multilineCaps, bool) {
	parts := strings.Fields(capLS)
	for _, part := range parts {
		name, value, hasValue := strings.Cut(part, "=")
		if name == capMultiline {
			if !hasValue {
				return multilineCaps{}, true
			}
			return parseMultilineCaps(value), true
		}
	}
	return multilineCaps{}, false
}

func isMultilineFail(e girc.Event) (code string, ok bool) {
	if e.Command != "FAIL" || len(e.Params) < 2 {
		return "", false
	}
	if e.Params[0] != cmdBATCH {
		return "", false
	}
	code = e.Params[1]
	switch code {
	case "MULTILINE_MAX_BYTES", "MULTILINE_MAX_LINES", "MULTILINE_INVALID_TARGET", "MULTILINE_INVALID":
		return code, true
	}
	return "", false
}

func formatFailMessage(e girc.Event) string {
	if len(e.Params) >= 3 {
		return fmt.Sprintf("multiline FAIL %s: %s", e.Params[1], e.Last())
	}
	return fmt.Sprintf("multiline FAIL %s", e.Params[1])
}
