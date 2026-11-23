package scylla

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const redacted = "REDACTED"

// secretKeyParts are parts of JSON keys whose values must not go to the log.
// The match is on purpose broad: it also hides harmless keys such as
// tokenType, because a hidden value costs less than a leaked secret.
var secretKeyParts = []string{"password", "secret", "token", "privatekey"}

// logBody returns an API body as the trace log shows it. In a JSON body, the
// values of secret keys are replaced. A body that is not valid JSON, such as a
// serverless connection bundle, can hold secrets that cannot be found, so
// only its size is shown.
func logBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()

	var v any
	if err := d.Decode(&v); err != nil || d.More() {
		if int64(len(body)) >= maxResponseBodyLength {
			return fmt.Sprintf("[%d bytes, cut at the size limit]", len(body))
		}
		return fmt.Sprintf("[%d bytes, not JSON]", len(body))
	}

	// Marshal cannot fail on a value that Decode made.
	out, _ := json.Marshal(redactValue(v))
	return string(out)
}

func redactValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, val := range v {
			if isSecretKey(k) {
				v[k] = redacted
			} else {
				v[k] = redactValue(val)
			}
		}
	case []any:
		for i := range v {
			v[i] = redactValue(v[i])
		}
	}
	return v
}

func isSecretKey(k string) bool {
	k = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(k))
	for _, p := range secretKeyParts {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}
