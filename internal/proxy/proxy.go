package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type RequestLog struct {
	Timestamp   time.Time     `json:"timestamp"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Headers     http.Header   `json:"headers"`
	RequestBody []byte        `json:"request_body"`
	Response    ResponseLog   `json:"response"`
	Duration    time.Duration `json:"duration"`
}

type ResponseLog struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

type Proxy struct {
	config       *Config
	requestStore *types.RequestStore
	logger       *zap.Logger
	client       *http.Client
	logs         chan RequestLog
}

type Config struct {
	BasePrefix string
	Target     string
}

func NewProxy(config *Config, requestStore *types.RequestStore) *Proxy {
	// config the encoder
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

	// create base encoding
	baseEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// wrap the encode with custom encoder
	prettyEncoder := &PrettyJSONEncoder{Encoder: baseEncoder}

	// Create in the core with custom encoder
	core := zapcore.NewCore(
		prettyEncoder,
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	// Crear el logger con el core
	logger := zap.New(core, zap.AddCaller())

	return &Proxy{
		config:       config,
		requestStore: requestStore,
		logger:       logger,
		client: &http.Client{
			Timeout: time.Duration(5*time.Second) * time.Second,
		},
		logs: make(chan RequestLog, 100),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	p.logger.Info("request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	//  + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix)
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

	var proxiedURL string
	if strings.HasPrefix(r.URL.Path, p.config.BasePrefix) {
		proxiedURL = targetURL.String() + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix)
	} else {
		p.logger.Error("La ruta no comienza con el BasePrefix esperado",
			zap.String("path", r.URL.Path),
			zap.String("basePrefix", p.config.BasePrefix),
		)
		http.Error(w, "Ruta no válida", http.StatusBadRequest)
		return
	}

	// Modify the director to capture the response and keep the path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		req.URL.Path = targetURL.Path + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix)

		p.logger.Info("request sending",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Any("headers", req.Header),
			zap.Any("query", req.URL.Query()),
		)
	}

	// Create the request log with the full target URL
	reqLog := &types.RequestLog{
		ID:        r.Header.Get("X-Request-ID"),
		Timestamp: time.Now(),
		Method:    r.Method,
		URL:       proxiedURL,
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
		logger:            p.logger,
	}

	proxy.ServeHTTP(w, r)

	// Registrar la solicitud
	log := RequestLog{
		Timestamp:   start,
		Method:      r.Method,
		Path:        r.URL.Path,
		Headers:     r.Header,
		RequestBody: reqLog.Body,
		Response: ResponseLog{
			StatusCode: 0, // This will be set in the response transport
			Headers:    nil,
			Body:       nil,
		},
		Duration: time.Since(start),
	}

	select {
	case p.logs <- log:
	default:
		// Si el canal está lleno, descartar el log
	}
}

type responseTransport struct {
	originalTransport http.RoundTripper
	requestLog        *types.RequestLog
	requestStore      *types.RequestStore
	logger            *zap.Logger
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
		zap.String("url", req.URL.String()),
	)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// Create response log
	t.requestLog.Response = &types.ResponseLog{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	// Store the request log
	t.requestStore.AddRequest(t.requestLog)

	return resp, nil
}

func (p *Proxy) GetLogs() <-chan RequestLog {
	return p.logs
}
