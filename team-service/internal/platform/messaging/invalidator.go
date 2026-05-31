package messaging

import (
	"context"
	"encoding/json"
	"log"

	redisCache "team-service/internal/platform/cache"
)

type teamActivityPayload struct {
	TeamID string `json:"teamId"`
}

func StartTeamCacheInvalidator(ctx context.Context, rabbitURL, exchange, queueName string, cache *redisCache.RedisCache) error {
	publisher, err := NewPublisher(rabbitURL, exchange)
	if err != nil {
		return err
	}
	defer publisher.Close()

	if err := publisher.EnsureQueue(queueName, "#"); err != nil {
		return err
	}

	msgs, err := publisher.Consume(queueName)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}

			var event Event
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				log.Printf("team cache invalidator: bad event: %v", err)
				continue
			}

			payload, _ := json.Marshal(event.Payload)
			var body teamActivityPayload
			if err := json.Unmarshal(payload, &body); err != nil {
				log.Printf("team cache invalidator: bad payload: %v", err)
				continue
			}
			if body.TeamID == "" {
				continue
			}
			if err := cache.DeleteMembers(ctx, body.TeamID); err != nil {
				log.Printf("team cache invalidator: delete cache: %v", err)
			}
		}
	}
}
