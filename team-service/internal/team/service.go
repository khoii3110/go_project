package team

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"team-service/internal/platform/auth"
	redisCache "team-service/internal/platform/cache"
	"team-service/internal/platform/messaging"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
)

type CreateTeamRequest struct {
	TeamName string `json:"teamName"`
}

type TeamUserRequest struct {
	UserID string `json:"userId"`
}

type Service interface {
	CreateTeam(ctx context.Context, actor auth.Principal, req CreateTeamRequest) (*Team, error)
	AddMember(ctx context.Context, actor auth.Principal, teamID string, req TeamUserRequest) (*Team, error)
	RemoveMember(ctx context.Context, actor auth.Principal, teamID, userID string) (*Team, error)
	AddManager(ctx context.Context, actor auth.Principal, teamID string, req TeamUserRequest) (*Team, error)
	ListMembers(ctx context.Context, actor auth.Principal, teamID string) ([]string, error)
}

type TeamService struct {
	repo      Repository
	cache     *redisCache.RedisCache
	publisher *messaging.Publisher
}

func NewService(repo Repository, cache *redisCache.RedisCache, publisher *messaging.Publisher) *TeamService {
	return &TeamService{repo: repo, cache: cache, publisher: publisher}
}

func (s *TeamService) CreateTeam(ctx context.Context, actor auth.Principal, req CreateTeamRequest) (*Team, error) {
	if actor.Role != "manager" {
		return nil, ErrForbidden
	}
	if strings.TrimSpace(req.TeamName) == "" {
		return nil, ErrInvalidInput
	}
	team, err := s.repo.CreateTeam(ctx, strings.TrimSpace(req.TeamName), actor.UserID)
	if err != nil {
		return nil, err
	}
	_ = s.publishTeamActivity(ctx, "TEAM_CREATED", map[string]any{
		"teamId":   team.TeamID,
		"teamName": team.TeamName,
		"actorId":  actor.UserID,
	})
	return team, nil
}

func (s *TeamService) AddMember(ctx context.Context, actor auth.Principal, teamID string, req TeamUserRequest) (*Team, error) {
	if actor.Role != "manager" {
		return nil, ErrForbidden
	}
	if _, err := uuid.Parse(teamID); err != nil {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		return nil, ErrInvalidInput
	}

	team, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !contains(team.Managers, actor.UserID) {
		return nil, ErrForbidden
	}

	updated, err := s.repo.AddMember(ctx, teamID, strings.TrimSpace(req.UserID))
	if err != nil {
		return nil, err
	}
	_ = s.invalidateMemberCache(ctx, teamID)
	_ = s.publishTeamActivity(ctx, "MEMBER_ADDED", map[string]any{
		"teamId":     teamID,
		"memberId":   strings.TrimSpace(req.UserID),
		"actorId":    actor.UserID,
		"event":      "team.member.added",
		"publishedAt": time.Now().UTC().Format(time.RFC3339Nano),
	})
	return updated, nil
}

func (s *TeamService) RemoveMember(ctx context.Context, actor auth.Principal, teamID, userID string) (*Team, error) {
	if actor.Role != "manager" {
		return nil, ErrForbidden
	}
	if _, err := uuid.Parse(teamID); err != nil {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrInvalidInput
	}
	team, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !contains(team.Managers, actor.UserID) {
		return nil, ErrForbidden
	}

	updated, err := s.repo.RemoveMember(ctx, teamID, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	_ = s.invalidateMemberCache(ctx, teamID)
	return updated, nil
}

func (s *TeamService) AddManager(ctx context.Context, actor auth.Principal, teamID string, req TeamUserRequest) (*Team, error) {
	if actor.Role != "manager" {
		return nil, ErrForbidden
	}
	if _, err := uuid.Parse(teamID); err != nil {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, ErrInvalidInput
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		return nil, ErrInvalidInput
	}

	team, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team.CreatorID != actor.UserID {
		return nil, ErrForbidden
	}

	return s.repo.AddManager(ctx, teamID, strings.TrimSpace(req.UserID))
}

func (s *TeamService) ListMembers(ctx context.Context, actor auth.Principal, teamID string) ([]string, error) {
	if actor.Role != "manager" {
		return nil, ErrForbidden
	}
	if _, err := uuid.Parse(teamID); err != nil {
		return nil, ErrInvalidInput
	}

	team, err := s.repo.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !contains(team.Managers, actor.UserID) {
		return nil, ErrForbidden
	}

	if s.cache != nil {
		if members, ok, err := s.cache.GetMembers(ctx, teamID); err == nil && ok {
			return members, nil
		}
	}

	members, err := s.repo.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SetMembers(ctx, teamID, members)
	}
	return members, nil
}

func (s *TeamService) invalidateMemberCache(ctx context.Context, teamID string) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.DeleteMembers(ctx, teamID)
}

func (s *TeamService) publishTeamActivity(ctx context.Context, eventType string, payload any) error {
	if s.publisher == nil {
		return nil
	}
	routingKey := map[string]string{
		"TEAM_CREATED":  "team.created",
		"MEMBER_ADDED":  "team.member.added",
		"MEMBER_REMOVED": "team.member.removed",
	}[eventType]
	if routingKey == "" {
		routingKey = strings.ToLower(eventType)
	}
	return s.publisher.Publish(ctx, routingKey, messaging.Event{
		SourceService: "team-service",
		EventType:     eventType,
		Payload:       payload,
	})
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
