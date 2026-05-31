package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrInvalidToken  = errors.New("invalid token")
	ErrMissingSecret = errors.New("missing jwt secret")
)

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type Service interface {
	Register(ctx context.Context, req CreateUserRequest) (*User, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID string) error
	ListUsers(ctx context.Context) ([]User, error)
	Authenticate(ctx context.Context, rawToken string) (*AuthContext, error)
}

type AuthService struct {
	repo         Repository
	jwtSecret    []byte
	jwtTTL       time.Duration
	tokenIssuer  string
	issuedAtFunc func() time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int64  `json:"expiresIn"`
}

type AuthContext struct {
	UserID    string
	Role      string
	SessionID string
}

type AuthClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewService(repo Repository, jwtSecret string, tokenIssuer string, jwtTTL time.Duration) (*AuthService, error) {
	if strings.TrimSpace(jwtSecret) == "" {
		return nil, ErrMissingSecret
	}
	if jwtTTL <= 0 {
		jwtTTL = time.Hour
	}
	if strings.TrimSpace(tokenIssuer) == "" {
		tokenIssuer = "auth-service"
	}

	return &AuthService{
		repo:         repo,
		jwtSecret:    []byte(jwtSecret),
		jwtTTL:       jwtTTL,
		tokenIssuer:  tokenIssuer,
		issuedAtFunc: time.Now,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, req CreateUserRequest) (*User, error) {
	if strings.TrimSpace(req.Username) == "" {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(req.Email) == "" {
		return nil, ErrInvalidInput
	}
	if len(req.Password) < 8 {
		return nil, ErrInvalidInput
	}
	if req.Role != RoleManager && req.Role != RoleMember {
		return nil, ErrInvalidInput
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return s.repo.CreateUser(ctx, CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		Role:         req.Role,
		PasswordHash: string(hashed),
	})
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, ErrInvalidInput
	}

	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrUnauthorized
	}

	now := s.issuedAtFunc().UTC()
	expiresAt := now.Add(s.jwtTTL)
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	claims := AuthClaims{
		UserID: u.UserID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.UserID,
			Issuer:    s.tokenIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        sessionID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreateSession(ctx, sessionID, u.UserID, expiresAt); err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.jwtTTL.Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrInvalidToken
	}
	return s.repo.RevokeSession(ctx, sessionID)
}

func (s *AuthService) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (*AuthContext, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidToken
	}

	claims := &AuthClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.ID == "" || claims.UserID == "" || claims.Role == "" {
		return nil, ErrInvalidToken
	}

	active, err := s.repo.IsSessionActive(ctx, claims.ID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrUnauthorized
	}

	return &AuthContext{
		UserID:    claims.UserID,
		Role:      claims.Role,
		SessionID: claims.ID,
	}, nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
