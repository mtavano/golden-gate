package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// redactedHeaders lists header names whose values must be replaced with
// [REDACTED] when persisting RequestLog. They are still sent to the upstream
// untouched — redaction only applies to the audit copy.
var redactedHeaders = map[string]struct{}{
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"proxy-authorization": {},
	"x-api-key":           {},
}

type Proxy struct {
	config     *Config
	requestSvc *service.RequestSvc
	logger     *zap.Logger
	client     *http.Client
}

type Config struct {
	ServiceName  string
	BasePrefix   string
	Target       string
	MaxBodyBytes int
}

func NewProxy(config *Config, requestSvc *service.RequestSvc) *Proxy {
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = defaultMaxBodyBytes
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	baseEncoder := zapcore.NewJSONEncoder(encoderConfig)
	prettyEncoder := &PrettyJSONEncoder{Encoder: baseEncoder}

	core := zapcore.NewCore(
		prettyEncoder,
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	logger := zap.New(core, zap.AddCaller())

	return &Proxy{
		config:     config,
		requestSvc: requestSvc,
		logger:     logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.logger.Info("request received",
		zap.String("service", p.config.ServiceName),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	targetURL, err := url.Parse(p.config.Target)
	if err != nil {
		p.logger.Error("invalid target URL", zap.Error(err))
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(r.URL.Path, p.config.BasePrefix) {
		p.logger.Error("path does not match expected BasePrefix",
			zap.String("path", r.URL.Path),
			zap.String("basePrefix", p.config.BasePrefix),
		)
		http.Error(w, "Ruta no válida", http.StatusBadRequest)
		return
	}

	proxiedURL := targetURL.String() + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix)

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := reverseProxy.Director
	reverseProxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		req.URL.Path = targetURL.Path + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix)

		// Force upstream to respond with an identity (uncompressed) body so the
		// stored audit copy is always human-readable. Without this, upstreams
		// behind Cloudflare may respond with brotli/zstd/etc which we cannot
		// decode here, leaving the dashboard with binary garbage.
		req.Header.Del("Accept-Encoding")

		p.logger.Info("request sending",
			zap.String("service", p.config.ServiceName),
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
		)
	}

	reqLog := &models.RequestLog{
		ServiceName: p.config.ServiceName,
		Timestamp:   time.Now(),
		Method:      r.Method,
		URL:         proxiedURL,
		Headers:     redactHeaders(r.Header),
		Query:       r.URL.Query(),
		CreatedAt:   time.Now(),
	}

	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			reqLog.Body, reqLog.BodyTruncated = truncate(body, p.config.MaxBodyBytes)
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	reverseProxy.Transport = &responseTransport{
		originalTransport: identityTransport(),
		requestLog:        reqLog,
		requestSvc:        p.requestSvc,
		logger:            p.logger,
		maxBodyBytes:      p.config.MaxBodyBytes,
		startedAt:         time.Now(),
	}

	reverseProxy.ServeHTTP(w, r)
}

type responseTransport struct {
	originalTransport http.RoundTripper
	requestLog        *models.RequestLog
	requestSvc        *service.RequestSvc
	logger            *zap.Logger
	maxBodyBytes      int
	startedAt         time.Time
}

func (t *responseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.originalTransport.RoundTrip(req)
	if err != nil {
		t.logger.Error("failed to send request", zap.Error(err))
		return nil, err
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	// Decompress gzip responses so the persisted body is human-readable JSON
	// and so the body shown to the client matches what we store.
	decoded := rawBody
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding == "gzip" {
		if d, derr := gunzip(rawBody); derr == nil {
			decoded = d
			resp.Header.Del("Content-Encoding")
			resp.Header.Set("Content-Length", strconv.Itoa(len(decoded)))
		} else {
			t.logger.Warn("failed to decompress gzip response, falling back to raw bytes",
				zap.Error(derr),
			)
		}
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(decoded))

	storedBody, truncated := truncate(decoded, t.maxBodyBytes)

	t.requestLog.DurationMs = time.Since(t.startedAt).Milliseconds()
	t.requestLog.Response = &models.ResponseLog{
		StatusCode:    resp.StatusCode,
		Headers:       redactHeaders(resp.Header),
		Body:          storedBody,
		BodyTruncated: truncated,
	}

	t.logger.Info("response received",
		zap.String("service", t.requestLog.ServiceName),
		zap.Int("status", resp.StatusCode),
		zap.Int64("duration_ms", t.requestLog.DurationMs),
		zap.String("url", req.URL.String()),
	)

	if err := t.requestSvc.AddRequest(t.requestLog); err != nil {
		t.logger.Error("failed to store request log", zap.Error(err))
	}

	return resp, nil
}

// identityTransport returns an http.RoundTripper that will NOT transparently
// add "Accept-Encoding: gzip" to outbound requests. Combined with stripping
// Accept-Encoding in the Director, this guarantees the upstream always answers
// with an identity (uncompressed) body, so the persisted audit copy is
// human-readable regardless of which codecs (gzip, br, zstd, ...) the
// upstream supports.
func identityTransport() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	t := base.Clone()
	t.DisableCompression = true
	return t
}

// gunzip decompresses a gzip-encoded payload.
func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// truncate caps body to max bytes. Returns the (possibly cut) body and a flag
// indicating whether truncation occurred.
func truncate(body []byte, max int) ([]byte, bool) {
	if max <= 0 || len(body) <= max {
		return body, false
	}
	out := make([]byte, max)
	copy(out, body[:max])
	return out, true
}

// redactHeaders returns a copy of h with sensitive header values replaced by
// [REDACTED]. The original header map is left untouched so the upstream still
// receives the real values.
func redactHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		if _, sensitive := redactedHeaders[strings.ToLower(k)]; sensitive {
			out[k] = []string{"[REDACTED]"}
			continue
		}
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}
