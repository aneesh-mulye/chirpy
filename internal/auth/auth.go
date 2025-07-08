package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const cost = 10
const issuer = "chirpy"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("error hashing password: %w", err)
	}
	return string(hash), nil
}

func CheckPasswordHash(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	switch {
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return fmt.Errorf("password does not match")
	case err != nil:
		return fmt.Errorf("invalid hash or comparison failed: %w", err)
	}
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string,
	expiresIn time.Duration) (string, error) {

	timeNow := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(timeNow),
		ExpiresAt: jwt.NewNumericDate(timeNow.Add(expiresIn)),
		Subject:   userID.String(),
	})
	ss, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", fmt.Errorf("error creating JWT token: %w", err)
	}
	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{},
		func(token *jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid JWT: %w", err)
	}
	if token.Method != jwt.SigningMethodHS256 {
		return uuid.UUID{}, fmt.Errorf("unexpected signing method: %v",
			token.Header["alg"])
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.UUID{}, errors.New("invalid 'claims' in JWT")
	}
	if claims.Issuer != issuer {
		return uuid.UUID{}, fmt.Errorf("invalid issuer: %v", claims.Issuer)
	}
	subjectUUID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("cannot parse UUID: %w", err)
	}
	return subjectUUID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authorization, ok := headers["Authorization"]
	if !ok {
		return "", errors.New("no Authorization header found")
	}
	for _, authForm := range authorization {
		if strings.HasPrefix(authForm, "Bearer ") {
			return strings.Trim(strings.TrimPrefix(authForm, "Bearer "), " \t\r\n"), nil
		}
	}
	return "", errors.New("no bearer token found")
}
