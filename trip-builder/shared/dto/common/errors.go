package common

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func NewError(err string, details string) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Details: details,
	}
}