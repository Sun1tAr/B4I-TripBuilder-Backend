package chat

type SendMessageRequest struct {
	Text string `json:"text" validate:"required,max=5000"`
}

type UpdateMessageRequest struct {
	Text string `json:"text" validate:"required,max=5000"`
}

type ListMessagesRequest struct {
	Page int `json:"page"`
	Size int `json:"size"`
}
