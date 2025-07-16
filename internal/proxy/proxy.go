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

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Proxy struct {
	config     *Config
	requestSvc *service.RequestSvc
	logger     *zap.Logger
	client     *http.Client
}

type Config struct {
	BasePrefix string
	Target     string
}

func NewProxy(config *Config, requestSvc *service.RequestSvc) *Proxy {
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
		config:     config,
		requestSvc: requestSvc,
		logger:     logger,
		client: &http.Client{
			Timeout: time.Duration(5*time.Second) * time.Second,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	reqLog := &models.RequestLog{
		ID:        0, // Will be set by database
		Timestamp: time.Now(),
		Method:    r.Method,
		URL:       proxiedURL,
		Headers:   r.Header,
		Query:     r.URL.Query(),
		CreatedAt: time.Now(),
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
		requestSvc:        p.requestSvc,
		logger:            p.logger,
	}

	proxy.ServeHTTP(w, r)
}

type responseTransport struct {
	originalTransport http.RoundTripper
	requestLog        *models.RequestLog
	requestSvc        *service.RequestSvc
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
	t.requestLog.Response = &models.ResponseLog{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	// Store the request log
	if err := t.requestSvc.AddRequest(t.requestLog); err != nil {
		t.logger.Error("failed to store request log",
			zap.Error(err),
		)
	}

	return resp, nil
}
