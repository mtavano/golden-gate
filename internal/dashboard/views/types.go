package views

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/models"
)

// ServiceCard renders one entry of the dashboard landing grid.
type ServiceCard struct {
	Name          string
	BasePrefix    string
	Target        string
	Count         int64
	LastRequestAt *time.Time
	Orphan        bool // data exists with no matching configured service
}

// ExploreView holds everything needed to render the explore page.
type ExploreView struct {
	ServiceName string
	BasePrefix  string
	Target      string
	Requests    []*models.RequestLog
	Count       int64
	From        time.Time // start of the time range in the display TZ
	To          time.Time // end of the time range in the display TZ
	Page        int
	PageSize    int
	HasNext     bool
	Location    *time.Location
}

// formatTime renders an absolute time in the dashboard timezone.
func formatTime(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

// formatTimeOptional renders a *time.Time or "—" if nil.
func formatTimeOptional(t *time.Time, loc *time.Location) string {
	if t == nil {
		return "—"
	}
	return formatTime(*t, loc)
}

// formatLocalInput renders a time as the value of an <input type="datetime-local">.
func formatLocalInput(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02T15:04")
}

func formatHeaders(headers map[string][]string) string {
	if len(headers) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(v, ", "))
		b.WriteByte('\n')
	}
	return b.String()
}

func formatQueryParams(params map[string][]string) string {
	if len(params) == 0 {
		return ""
	}
	var b strings.Builder
	for k, v := range params {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(v, ", "))
		b.WriteByte('\n')
	}
	return b.String()
}

// contentTypeOf extracts the lowercase media type (without parameters) from a
// headers map. Returns "" if absent.
func contentTypeOf(headers map[string][]string) string {
	if headers == nil {
		return ""
	}
	for k, vs := range headers {
		if !strings.EqualFold(k, "Content-Type") || len(vs) == 0 {
			continue
		}
		ct := vs[0]
		if idx := strings.Index(ct, ";"); idx != -1 {
			ct = ct[:idx]
		}
		return strings.TrimSpace(strings.ToLower(ct))
	}
	return ""
}

// formatBodySmart renders a body based on the declared Content-Type rather
// than a printable-bytes heuristic. JSON is pretty-printed, text passes
// through, binary collapses to a one-line placeholder. Truncation is reported
// when the caller flagged the body as truncated.
func formatBodySmart(body []byte, contentType string, truncated bool) string {
	if len(body) == 0 {
		return "[Empty body]"
	}

	prefix := ""
	if truncated {
		prefix = fmt.Sprintf("[Truncated at %d bytes — full body was larger]\n\n", len(body))
	}

	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		ct = http.DetectContentType(body)
		if idx := strings.Index(ct, ";"); idx != -1 {
			ct = strings.TrimSpace(strings.ToLower(ct[:idx]))
		}
	}

	if strings.Contains(ct, "json") {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err == nil {
			return prefix + pretty.String()
		}
		return prefix + string(body)
	}

	if strings.HasPrefix(ct, "text/") || strings.Contains(ct, "xml") || strings.Contains(ct, "javascript") || strings.Contains(ct, "html") || strings.Contains(ct, "x-www-form-urlencoded") {
		return prefix + string(body)
	}

	return fmt.Sprintf("%s[Binary content — %s, %d bytes]", prefix, ct, len(body))
}

func buildCurlCommand(req *models.RequestLog) string {
	cmd := []string{"curl", "-X", req.Method, "'" + req.URL + "'"}
	for k, vs := range req.Headers {
		for _, v := range vs {
			cmd = append(cmd, "-H", "'"+k+": "+v+"'")
		}
	}
	if len(req.Body) > 0 {
		cmd = append(cmd, "--data-binary", "'"+escapeSingleQuotes(string(req.Body))+"'")
	}
	return strings.Join(cmd, " ")
}

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

func statusClass(code int) string {
	switch {
	case code == 0:
		return "bg-gray-100 text-gray-700"
	case code >= 200 && code < 300:
		return "bg-green-100 text-green-800"
	case code >= 300 && code < 400:
		return "bg-blue-100 text-blue-800"
	case code >= 400 && code < 500:
		return "bg-yellow-100 text-yellow-800"
	default:
		return "bg-red-100 text-red-800"
	}
}
