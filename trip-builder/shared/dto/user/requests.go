package user

import (
	"github.com/google/uuid"
)

type UpdateProfileRequest struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Language  *string `json:"language,omitempty"`
	Timezone  *string `json:"timezone,omitempty"`
	NotificationsEnabled *bool `json:"notifications_enabled,omitempty"`
}

type FollowRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

type PaginationRequest struct {
	Limit  int `json:"limit" validate:"min=1,max=100"`
	Offset int `json:"offset" validate:"min=0"`
}