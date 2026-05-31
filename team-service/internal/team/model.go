package team

import "time"

type Team struct {
	TeamID    string    `json:"teamId"`
	TeamName  string    `json:"teamName"`
	CreatorID string    `json:"creatorId"`
	Managers  []string  `json:"managers"`
	Members   []string  `json:"members"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
