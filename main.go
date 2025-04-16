package main

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	var apiCfg apiConfig
	srv := http.Server{}
	srv.Addr = ":8080"
	smux := http.NewServeMux()
	smux.Handle("/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix(
			"/app", http.FileServer(http.Dir("./app/")))))
	smux.HandleFunc("POST /reset", apiCfg.handlerReset)
	smux.HandleFunc("GET /metrics", apiCfg.handlerMetrics)
	smux.HandleFunc("GET /healthz",
		func(rw http.ResponseWriter, req *http.Request) {
			rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
			rw.WriteHeader(200)
			_, err := rw.Write([]byte("OK"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
			}
		})
	srv.Handler = smux
	err := srv.ListenAndServe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "server error: %s", err.Error())
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, req)
	})
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, _ *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
	}
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
	}
}
