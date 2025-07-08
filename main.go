package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
}

const maxChirpLength = 140
const defaultExpiryInSeconds = 3600

func main() {
	var apiCfg apiConfig

	err := godotenv.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't load .env file: %s", err.Error())
		return
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't conect to DB: %s", err.Error())
		return
	}
	apiCfg.db = database.New(db)
	apiCfg.platform = os.Getenv("PLATFORM")
	apiCfg.secret = os.Getenv("CHIRPY_SECRET")

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
			_, err := rw.Write([]byte("OK"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
				rw.WriteHeader(500)
			}
			rw.WriteHeader(200)
		})
	smux.HandleFunc("POST /api/validate_chirp", handlerValidate)
	smux.HandleFunc("POST /api/users", apiCfg.handlerUseradd)
	smux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	smux.HandleFunc("POST /api/chirps", apiCfg.handlerChirpadd)
	smux.HandleFunc("GET /api/chirps", apiCfg.handlerAllChirps)
	smux.HandleFunc("GET /api/chirps/{id}", apiCfg.handlerChirp)
	srv.Handler = smux
	err = srv.ListenAndServe()
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

func (cfg *apiConfig) handlerChirp(w http.ResponseWriter, r *http.Request) {
	type Chirp struct {
		ID         string    `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Body       string    `json:"body"`
		UserID     string    `json:"user_id"`
	}
	// Get the ID
	reqID := r.PathValue("id")
	// Make sure it's a valid UUID
	chirpID, err := uuid.Parse(reqID)
	if err != nil {
		errorStr := fmt.Sprintf("Not a valid Chirp ID (UUID): %s: %s",
			reqID, err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 400)
		return
	}
	// DB query
	dbchirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		errorStr := fmt.Sprintf("Error fetching Chirp: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}
	// Send response
	err = respondWithJSON(w, 200,
		Chirp{
			ID:         dbchirp.ID.String(),
			Created_at: dbchirp.CreatedAt,
			Updated_at: dbchirp.UpdatedAt,
			Body:       dbchirp.Body,
			UserID:     dbchirp.UserID.String(),
		})
	if err != nil {
		errorStr := fmt.Sprintf("Error responding: %s", err.Error())
		log.Println(errorStr)
	}
}

func (cfg *apiConfig) handlerAllChirps(w http.ResponseWriter, r *http.Request) {
	type Chirp struct {
		ID         string    `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Body       string    `json:"body"`
		UserID     string    `json:"user_id"`
	}

	dbchirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		errorStr := fmt.Sprintf("Error fetching chirps: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}

	chirps := []Chirp{}
	for _, dbchirp := range dbchirps {
		chirps = append(chirps,
			Chirp{
				ID:         dbchirp.ID.String(),
				Created_at: dbchirp.CreatedAt,
				Updated_at: dbchirp.UpdatedAt,
				Body:       dbchirp.Body,
				UserID:     dbchirp.UserID.String(),
			})
	}

	err = respondWithJSON(w, 200, chirps)
	if err != nil {
		errorStr := fmt.Sprintf("Error responding: %s", err.Error())
		log.Println(errorStr)
	}
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type LoginReq struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	type LoginResponse struct {
		ID         string    `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Email      string    `json:"email"`
		Token      string    `json:"token"`
	}

	// Parse the request
	decoder := json.NewDecoder(r.Body)
	request := LoginReq{}
	err := decoder.Decode(&request)
	if err != nil {
		errorStr := fmt.Sprintf("Error decoding parameters: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}
	if request.ExpiresInSeconds <= 0 ||
		request.ExpiresInSeconds > defaultExpiryInSeconds {
		request.ExpiresInSeconds = defaultExpiryInSeconds
	}

	// Get the password hash of the password from the request.
	//reqPasswordHash, err := auth.HashPassword(request.Password)
	//if err != nil {
	//	log.Printf("Password from request cannot be hashed: %s", err.Error())
	//	http.Error(w, "Incorrect email or password.", http.StatusUnauthorized)
	//	return
	//}
	// Get the user from the DB.
	storedUser, err := cfg.db.GetUserByEmail(r.Context(), request.Email)
	if err != nil {
		log.Printf("Error fetching user from DB: %s", err.Error())
		http.Error(w, "Incorrect email or password.", http.StatusUnauthorized)
		return
	}
	// Check if the password hashes match, and respond with 401 if they don't.
	err = auth.CheckPasswordHash(storedUser.HashedPassword, request.Password)
	if err != nil {
		log.Println("Password hashes don't match in login attempt.")
		http.Error(w, "Incorrect email or password.", http.StatusUnauthorized)
		return
	}

	jwt, err := auth.MakeJWT(storedUser.ID, cfg.secret,
		time.Second*time.Duration(request.ExpiresInSeconds))
	if err != nil {
		log.Println("Couldn't generate JWT") // This should have more info.
		http.Error(w, "Auth failed but because of us, not you.",
			http.StatusInternalServerError)
		return
	}

	response := LoginResponse{
		ID:         storedUser.ID.String(),
		Created_at: storedUser.CreatedAt,
		Updated_at: storedUser.UpdatedAt,
		Email:      storedUser.Email,
		Token:      jwt,
	}
	err = respondWithJSON(w, http.StatusOK, response)
	if err != nil {
		errorStr := fmt.Sprintf("Error responding: %s", err.Error())
		log.Println(errorStr)
	}
}

func (cfg *apiConfig) handlerUseradd(w http.ResponseWriter, r *http.Request) {
	type CUReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type Response struct {
		ID         string    `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Email      string    `json:"email"`
	}
	// First, parse the request
	decoder := json.NewDecoder(r.Body)
	request := CUReq{}
	err := decoder.Decode(&request)
	if err != nil {
		errorStr := fmt.Sprintf("Error decoding parameters: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}
	// Then, try to get a password hash.
	hashedPassword, err := auth.HashPassword(request.Password)
	if err != nil {
		errorStr := fmt.Sprintf("Error creating user: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 400)
		return
	}
	// Then, try to add the user
	createdUser, err := cfg.db.CreateUser(r.Context(),
		database.CreateUserParams{
			Email:          request.Email,
			HashedPassword: hashedPassword,
		})
	if err != nil {
		errorStr := fmt.Sprintf("Error creating user: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}
	response := Response{
		ID:         createdUser.ID.String(),
		Created_at: createdUser.CreatedAt,
		Updated_at: createdUser.UpdatedAt,
		Email:      createdUser.Email,
	}

	// Marshal newly created data into a JSON struct, and return it.
	err = respondWithJSON(w, 201, response)
	if err != nil {
		errorStr := fmt.Sprintf("Error responding: %s", err.Error())
		log.Println(errorStr)
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	err := cfg.db.Reset(r.Context())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
		w.WriteHeader(500)
		return
	}
	cfg.fileserverHits.Store(0)
	w.WriteHeader(200)
	_, err = w.Write([]byte("OK"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
		w.WriteHeader(500)
		return
	}

}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(fmt.Appendf([]byte{},
		metricsTemplate, cfg.fileserverHits.Load()))
	w.WriteHeader(200)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing response: %s", err.Error())
	}
}

func (cfg *apiConfig) handlerChirpadd(w http.ResponseWriter, r *http.Request) {
	type CCReq struct {
		Body string `json:"body"`
	}
	type Chirp struct {
		ID         string    `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Body       string    `json:"body"`
		UserID     string    `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	request := CCReq{}
	err := decoder.Decode(&request)
	if err != nil {
		errorStr := fmt.Sprintf("Error decoding parameters: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Println("Error getting bearer token: " + err.Error())
		http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Println("Error validating JWT: " + err.Error())
		http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
		return
	}

	if valid, err := isChirpValid(request.Body); !valid {
		errStr := fmt.Sprintf("chirp is not valid: %s", err.Error())
		log.Println(errStr)
		http.Error(w, errStr, 400)
		return
	}

	// Now directly try to write to the db? And if that fails, just return an
	// error — but of what kind?
	// TODO: not distinquishing currently between user already exists, and
	// some other DB problem. Fixing this sometime, maybe.
	createdChirp, err := cfg.db.CreateChirp(r.Context(),
		database.CreateChirpParams{
			Body:   request.Body,
			UserID: userID,
		})
	if err != nil {
		errorStr := fmt.Sprintf("Error creating chirp: %s", err.Error())
		log.Println(errorStr)
		http.Error(w, errorStr, 500)
		return
	}
	response := Chirp{
		ID:         createdChirp.ID.String(),
		Created_at: createdChirp.CreatedAt,
		Updated_at: createdChirp.UpdatedAt,
		Body:       createdChirp.Body,
		UserID:     createdChirp.UserID.String(),
	}

	// Marshal newly created data into a JSON struct, and return it.
	err = respondWithJSON(w, 201, response)
	if err != nil {
		errorStr := fmt.Sprintf("Error responding: %s", err.Error())
		log.Println(errorStr)
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
	dat, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("couldn't marshal JSON: %w", err)
	}
	w.WriteHeader(code)
	_, err = w.Write(dat)
	if err != nil {
		w.WriteHeader(500)
		return fmt.Errorf("error writing to response: %w", err)
	}

	return nil
}

func splitWithSpaces(s string) []string {
	var beg int
	res := make([]string, 0)
	if len(s) == 0 {
		return res
	}
	var inWord bool
	if s[0] != ' ' {
		inWord = true
	}
	for i, r := range s {
		if (inWord && r != ' ') || (!inWord && r == ' ') {
			continue
		}
		res = append(res, s[beg:i])
		beg = i
		inWord = !inWord
	}
	res = append(res, s[beg:])

	return res
}

func isChirpValid(chirp string) (bool, error) {
	if !isChirpLengthValid(chirp) {
		return false, errors.New("chirp of invalid length")
	}
	if !isChirpClean(chirp) {
		return false, errors.New("chirp is scandalous(!)")
	}
	return true, nil
}

func isChirpClean(susChirp string) bool {
	susWords := splitWithSpaces(susChirp)
	for _, word := range susWords {
		if theProfane[strings.ToLower(word)] {
			return false
		}
	}
	return true
}

func isChirpLengthValid(chirp string) bool {
	return len(chirp) <= maxChirpLength
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
