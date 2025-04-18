package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	smux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	smux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	smux.HandleFunc("GET /api/healthz",
		func(rw http.ResponseWriter, req *http.Request) {
			rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
			rw.WriteHeader(200)
			_, err := rw.Write([]byte("OK"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
			}
		})
	smux.HandleFunc("POST /api/validate_chirp", handlerValidate)
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
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write(fmt.Appendf([]byte{},
		metricsTemplate, cfg.fileserverHits.Load()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
	}
}

func handlerValidate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	type validResponse struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		errorStr := fmt.Sprintf("Error decoding parameters: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}

	if len(params.Body) <= 140 {
		err = respondWithJSON(w, 200, validResponse{CleanedBody: cleanupChirp(params.Body)})
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		if err = respondWithJSON(w, 400, errorResponse{Error: "Chirp is too long"}); err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
	}
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	dat, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Couldn't marshal JSON: %w", err)
	}
	_, err = w.Write(dat)
	if err != nil {
		return fmt.Errorf("Error writing to response: %w", err)
	}

	return nil
}

func splitWithSpaces(s string) []string {
	var beg int
	res := make([]string, 0)
	if 0 == len(s) {
		return res
	}
	var inWord bool
	if ' ' != s[0] {
		inWord = true
	}
	for i, r := range s {
		if (inWord && ' ' != r) || (!inWord && ' ' == r) {
			continue
		}
		res = append(res, s[beg:i])
		beg = i
		inWord = !inWord
	}
	res = append(res, s[beg:])

	return res
}

func cleanupChirp(susChirp string) string {
	susWords := splitWithSpaces(susChirp)
	cleanWords := make([]string, 0, len(susWords))
	for _, word := range susWords {
		if theProfane[strings.ToLower(word)] {
			cleanWords = append(cleanWords, "****")
		} else {
			cleanWords = append(cleanWords, word)
		}
	}

	return strings.Join(cleanWords, "")
}

var theProfane = map[string]bool{
	"kerfuffle": true,
	"sharbert":  true,
	"fornax":    true,
}

var metricsTemplate = `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
