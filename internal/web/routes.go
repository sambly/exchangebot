package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

func init() {
	// Регистрируем метрики
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

func getFrontendAssets(production bool, content embed.FS) fs.FS {

	path := "frontend/dist"

	if production {
		fsys := fs.FS(content)
		f, err := fs.Sub(fsys, path)
		if err != nil {
			fmt.Println(err)
		}
		return f
	}
	return os.DirFS(path)
}

func (app *Web) routes() *http.ServeMux {

	mux := http.NewServeMux()

	// Промежуточный обработчик для измерения метрик
	instrumentedHandler := func(path string, next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			httpRequestsTotal.WithLabelValues(path, r.Method, fmt.Sprintf("%d", rec.statusCode)).Inc()
			httpRequestDuration.WithLabelValues(path).Observe(duration)
		}
	}

	mux.HandleFunc("/trade/formingPage", app.basicAuth(instrumentedHandler("/trade/formingPage", app.formingPage)))
	mux.HandleFunc("/trade/updatefull", app.basicAuth(instrumentedHandler("/trade/updatefull", app.updateFull)))
	mux.HandleFunc("/trade/getChangeDelta", app.basicAuth(instrumentedHandler("/trade/getChangeDelta", app.getDeltaFast)))
	mux.HandleFunc("/trade/updateTop", app.basicAuth(instrumentedHandler("/trade/updateTop", app.updateTop)))
	mux.HandleFunc("/trade/openDeal", app.basicAuth(instrumentedHandler("/trade/openDeal", app.openDeal)))
	mux.HandleFunc("/trade/closeDeal", app.basicAuth(instrumentedHandler("/trade/closeDeal", app.closeDeal)))
	// TODO пока закоментил, выдывало ошибку
	// mux.HandleFunc("/trade/ws", app.basicAuth(instrumentedHandler("/trade/ws", app.echo)))
	mux.HandleFunc("/trade/ws", app.basicAuth(app.echo))

	mux.HandleFunc("/trade/getChPrice", app.basicAuth(instrumentedHandler("/trade/getChPrice", app.getChPrice)))
	mux.HandleFunc("/trade/getChDelta", app.basicAuth(instrumentedHandler("/trade/getChDelta", app.getChDelta)))

	mux.HandleFunc("/trade/closeAllDeal", app.basicAuth(instrumentedHandler("/trade/closeAllDeal", app.closeAllDeal)))

	//mux.HandleFunc("/trade/grafana", app.basicAuth(instrumentedHandler("/trade/grafana", app.grafana)))
	mux.HandleFunc("/trade/grafana/", app.basicAuth(app.grafana))

	// Сервер статических файлов
	fileServer := http.FileServer(http.FS(getFrontendAssets(app.contentEmbed, app.content)))
	mux.HandleFunc("/trade/", app.basicAuth(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/trade", instrumentedHandler("/trade", fileServer.ServeHTTP)).ServeHTTP(w, r)
	}))

	// Экспонируем метрики на маршруте /metrics
	mux.Handle("/metrics", promhttp.Handler())

	return mux
}

func (app *Web) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		username, password, ok := r.BasicAuth()

		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(app.auth.username))
			expectedPasswordHash := sha256.Sum256([]byte(app.auth.password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {

				next.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}
