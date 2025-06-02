package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/types"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// PrettyJSONEncoder es un encoder personalizado que hace pretty print del JSON
type PrettyJSONEncoder struct {
	zapcore.Encoder
}

func (e *PrettyJSONEncoder) Clone() zapcore.Encoder {
	return &PrettyJSONEncoder{
		Encoder: e.Encoder.Clone(),
	}
}

func (e *PrettyJSONEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf, err := e.Encoder.EncodeEntry(ent, fields)
	if err != nil {
		return nil, err
	}

	// Hacer pretty print del JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}

	newBuf := buffer.NewPool().Get()
	newBuf.AppendString(prettyJSON.String() + "\n")
	return newBuf, nil
}

type Proxy struct {
	config      *Config
	requestStore *types.RequestStore
	logger      *zap.Logger
}

type Config struct {
	BasePrefix string
	Target     string
}

func NewProxy(config *Config, requestStore *types.RequestStore) *Proxy {
	// Configurar el encoder para pretty printing
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

	// Crear el encoder base
	baseEncoder := zapcore.NewJSONEncoder(encoderConfig)
	
	// Envolver el encoder base con nuestro encoder personalizado
	prettyEncoder := &PrettyJSONEncoder{Encoder: baseEncoder}
	
	// Crear el core con el encoder personalizado
	core := zapcore.NewCore(
		prettyEncoder,
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)
	
	// Crear el logger con el core
	logger := zap.New(core, zap.AddCaller())
	
	return &Proxy{
		config:      config,
		requestStore: requestStore,
		logger:      logger,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.logger.Info("request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Any("headers", r.Header),
	)
	
	targetURL, err := url.Parse(p.config.Target)
	if err != nil {
		p.logger.Error("invalid target URL",
			zap.Error(err),
		)
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	// Create the proxy director
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Modify the director to capture the response and keep the path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		
		// Get the path after the base_prefix
		path := strings.TrimPrefix(req.URL.Path, p.config.BasePrefix)
		req.URL.Path = path
		
		// Incluir los query params del request original
		req.URL.RawQuery = r.URL.RawQuery
		
		p.logger.Info("request sending",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Any("headers", req.Header),
		)
	}

	// Create the request log with the full target URL
	reqLog := &types.RequestLog{
		ID:        r.Header.Get("X-Request-ID"),
		Timestamp: time.Now(),
		Method:    r.Method,
		URL:       targetURL.String() + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix),
		Headers:   r.Header,
		Query:     r.URL.Query(),
	}

	// Read the request body
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			reqLog.Body = body
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	// Modify the transport to capture the response
	proxy.Transport = &responseTransport{
		originalTransport: http.DefaultTransport,
		requestLog:        reqLog,
		requestStore:      p.requestStore,
		logger:           p.logger,
	}

	proxy.ServeHTTP(w, r)
}

type responseTransport struct {
	originalTransport http.RoundTripper
	requestLog        *types.RequestLog
	requestStore      *types.RequestStore
	logger           *zap.Logger
}

func (t *responseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.originalTransport.RoundTrip(req)
	if err != nil {
		t.logger.Error("failed to send request",
			zap.Error(err),
		)
		return nil, err
	}

	t.logger.Info("response received",
		zap.Int("status", resp.StatusCode),
		zap.Any("headers", resp.Header),
	)

	// Create the response log
	respLog := &types.ResponseLog{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}

	// Read the response body
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			respLog.Body = body
			resp.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	t.requestLog.Response = respLog
	t.requestStore.AddRequest(t.requestLog)

	return resp, nil
} 