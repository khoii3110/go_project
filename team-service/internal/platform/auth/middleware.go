package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidToken = errors.New("invalid token")
)

type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Middleware struct {
	secret []byte
	issuer string
}

func NewMiddleware(secret, issuer string) *Middleware {
	if strings.TrimSpace(issuer) == "" {
		issuer = "auth-service"
	}
	return &Middleware{secret: []byte(secret), issuer: issuer}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeAuthError(w, ErrUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, ErrInvalidToken)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return m.secret, nil
		})
		if err != nil || !token.Valid {
			writeAuthError(w, ErrInvalidToken)
			return
		}
		if claims.UserID == "" || claims.Role == "" || claims.Issuer != m.issuer {
			writeAuthError(w, ErrInvalidToken)
			return
		}

		ctx := WithPrincipal(r.Context(), Principal{UserID: claims.UserID, Role: claims.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	status := http.StatusUnauthorized
	if errors.Is(err, ErrInvalidToken) {
		status = http.StatusUnauthorized
	}
	http.Error(w, http.StatusText(status), status)
}

func RequirePrincipal(ctx context.Context) (Principal, bool) {
	return PrincipalFromContext(ctx)
}
