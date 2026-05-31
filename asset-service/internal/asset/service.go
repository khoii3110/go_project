package asset

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"asset-service/internal/platform/cache"
	"asset-service/internal/platform/messaging"
	"github.com/google/uuid"
)

var ErrInvalidInput = errors.New("invalid input")

type Service struct {
	repo      Repository
	cache     *cache.RedisCache
	publisher *messaging.Publisher
}

func NewService(repo Repository, cache *cache.RedisCache, publisher *messaging.Publisher) *Service {
	return &Service{repo: repo, cache: cache, publisher: publisher}
}

func (s *Service) GetMetadata(ctx context.Context, assetID string) ([]byte, error) {
	if _, err := uuid.Parse(assetID); err != nil { return nil, ErrInvalidInput }
	if s.cache != nil {
		if cached, ok, err := s.cache.GetMetadata(ctx, assetID); err == nil && ok { return cached, nil }
	}
	asset, err := s.repo.GetMetadata(ctx, assetID)
	if err != nil { return nil, err }
	if s.cache != nil { _ = s.cache.SetMetadata(ctx, assetID, asset.Metadata) }
	return asset.Metadata, nil
}

func (s *Service) UpdateMetadata(ctx context.Context, assetID string, metadata json.RawMessage) ([]byte, error) {
	if _, err := uuid.Parse(assetID); err != nil { return nil, ErrInvalidInput }
	asset, err := s.repo.UpsertMetadata(ctx, assetID, metadata)
	if err != nil { return nil, err }
	if s.cache != nil { _ = s.cache.SetMetadata(ctx, assetID, asset.Metadata) }
	_ = s.publish(ctx, "NOTE_UPDATED", "note.updated", map[string]any{"assetId": assetID, "updatedAt": time.Now().UTC().Format(time.RFC3339Nano)})
	return asset.Metadata, nil
}

func (s *Service) GetACL(ctx context.Context, assetID string) ([]string, error) {
	if _, err := uuid.Parse(assetID); err != nil { return nil, ErrInvalidInput }
	if s.cache != nil {
		if cached, ok, err := s.cache.GetACL(ctx, assetID); err == nil && ok { return cached, nil }
	}
	asset, err := s.repo.GetACL(ctx, assetID)
	if err != nil { return nil, err }
	if s.cache != nil { _ = s.cache.SetACL(ctx, assetID, asset.ACL) }
	return asset.ACL, nil
}

func (s *Service) SetACL(ctx context.Context, assetID string, acl []string) ([]string, error) {
	if _, err := uuid.Parse(assetID); err != nil { return nil, ErrInvalidInput }
	asset, err := s.repo.SetACL(ctx, assetID, acl)
	if err != nil { return nil, err }
	if s.cache != nil { _ = s.cache.SetACL(ctx, assetID, asset.ACL) }
	return asset.ACL, nil
}

func (s *Service) ShareFolder(ctx context.Context, folderID string) error {
	if _, err := uuid.Parse(folderID); err != nil { return ErrInvalidInput }
	return s.publish(ctx, "FOLDER_SHARED", "folder.shared", map[string]any{"folderId": folderID, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Service) UpdateNote(ctx context.Context, noteID string) error {
	if _, err := uuid.Parse(noteID); err != nil { return ErrInvalidInput }
	return s.publish(ctx, "NOTE_UPDATED", "note.updated", map[string]any{"noteId": noteID, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Service) publish(ctx context.Context, eventType, routingKey string, payload any) error {
	if s.publisher == nil { return nil }
	return s.publisher.Publish(ctx, routingKey, messaging.Event{SourceService: "asset-service", EventType: eventType, Payload: payload})
}
