package user

import (
	"bytes"
	"encoding/csv"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"errors"
	"strings"
	"sync"
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
	ImportUsers(ctx context.Context, req ImportUsersRequest) (*ImportUsersSummary, error)
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

type ImportUsersRequest struct {
	Filename    string
	ContentType  string
	CSVContents  []byte
	UploadedByID string
}

type ImportUsersSummary struct {
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
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

type importJob struct {
	RowNumber int
	Record    []string
}

type importResult struct {
	RowNumber int
	Err       error
}

func (s *AuthService) ImportUsers(ctx context.Context, req ImportUsersRequest) (*ImportUsersSummary, error) {
	if len(bytes.TrimSpace(req.CSVContents)) == 0 {
		return nil, ErrInvalidInput
	}

	reader := csv.NewReader(bytes.NewReader(req.CSVContents))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, ErrInvalidInput
	}
	if len(records) == 0 {
		return nil, ErrInvalidInput
	}

	startIndex := 0
	if looksLikeHeader(records[0]) {
		startIndex = 1
	}
	if startIndex >= len(records) {
		return nil, ErrInvalidInput
	}

	jobs := make(chan importJob)
	results := make(chan importResult)
	const workerCount = 5

	var wg sync.WaitGroup
	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		wg.Add(1)

		// Each goroutine is a worker. A goroutine is a lightweight function that can run at the same time as other goroutines.
		// We use 5 workers here so several CSV rows can be processed concurrently.
		go func() {
			defer wg.Done()

			// The jobs channel is how the main loop sends CSV rows to workers.
			// Each worker keeps reading from the channel until it is closed.
			for job := range jobs {
				if ctx.Err() != nil {
					results <- importResult{RowNumber: job.RowNumber, Err: ctx.Err()}
					continue
				}

				req, parseErr := parseImportRecord(job.Record)
				if parseErr != nil {
					results <- importResult{RowNumber: job.RowNumber, Err: parseErr}
					continue
				}

				_, createErr := s.Register(ctx, req)
				results <- importResult{RowNumber: job.RowNumber, Err: createErr}
			}
		}()
	}

	go func() {
		// The main goroutine acts as the producer. It sends work items into the jobs channel.
		for index := startIndex; index < len(records); index++ {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- importJob{RowNumber: index + 1, Record: records[index]}:
			}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	summary := &ImportUsersSummary{Errors: make([]string, 0)}
	for result := range results {
		if result.Err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("row %d: %v", result.RowNumber, result.Err))
			continue
		}
		summary.Succeeded++
	}

	return summary, nil
}

func looksLikeHeader(record []string) bool {
	if len(record) < 4 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(record[0]))
	second := strings.ToLower(strings.TrimSpace(record[1]))
	third := strings.ToLower(strings.TrimSpace(record[2]))
	fourth := strings.ToLower(strings.TrimSpace(record[3]))
	return first == "username" && second == "email" && third == "role" && fourth == "password"
}

func parseImportRecord(record []string) (CreateUserRequest, error) {
	if len(record) < 4 {
		return CreateUserRequest{}, ErrInvalidInput
	}

	return CreateUserRequest{
		Username: strings.TrimSpace(record[0]),
		Email:    strings.TrimSpace(record[1]),
		Role:     strings.TrimSpace(record[2]),
		Password: strings.TrimSpace(record[3]),
	}, nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
