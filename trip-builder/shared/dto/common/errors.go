package common

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details string            `json:"details,omitempty"`
}

func NewError(err string, details string) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Details: details,
	}
}

func NewValidationError(fields map[string]string) *ValidationErrorResponse {
	return &ValidationErrorResponse{
		Error:  "validation failed",
		Fields: fields,
	}
}
