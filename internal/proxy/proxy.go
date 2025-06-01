package proxy

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/models"
)

type Proxy struct {
	config      *Config
	requestStore *models.RequestStore
}

type Config struct {
	BasePrefix string
	Target     string
}

func NewProxy(config *Config, requestStore *models.RequestStore) *Proxy {
	return &Proxy{
		config:      config,
		requestStore: requestStore,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(p.config.Target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	// Crear el director del proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Modificar el director para capturar la respuesta y mantener la ruta
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		
		// Obtener la ruta después del base_prefix
		path := strings.TrimPrefix(req.URL.Path, p.config.BasePrefix)
		req.URL.Path = path
	}

	// Crear el log del request con la URL completa del target
	reqLog := &models.RequestLog{
		ID:        r.Header.Get("X-Request-ID"),
		Timestamp: time.Now(),
		Method:    r.Method,
		URL:       targetURL.String() + strings.TrimPrefix(r.URL.Path, p.config.BasePrefix),
		Headers:   r.Header,
		Query:     r.URL.Query(),
	}

	// Leer el body del request
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			reqLog.Body = body
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	// Modificar el transport para capturar la respuesta
	proxy.Transport = &responseTransport{
		originalTransport: http.DefaultTransport,
		requestLog:        reqLog,
		requestStore:      p.requestStore,
	}

	proxy.ServeHTTP(w, r)
}

type responseTransport struct {
	originalTransport http.RoundTripper
	requestLog        *models.RequestLog
	requestStore      *models.RequestStore
}

func (t *responseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.originalTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Crear el log de la respuesta
	respLog := &models.ResponseLog{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}

	// Leer el body de la respuesta
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