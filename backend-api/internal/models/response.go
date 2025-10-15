package models

import "time"

// StandardResponse is the unified API envelope for all endpoints
type StandardResponse struct {
	Data    interface{}      `json:"data"`              // List, object, or null
	Summary interface{}      `json:"summary,omitempty"` // Optional aggregates
	Meta    ResponseMeta     `json:"meta"`              // Pagination, timing
	Error   *ResponseError   `json:"error,omitempty"`   // Error details (null on success)
}

// ResponseMeta contains pagination and request metadata
type ResponseMeta struct {
	Limit       int       `json:"limit,omitempty"`        // Requested limit
	Offset      int       `json:"offset,omitempty"`       // Requested offset
	Returned    int       `json:"returned"`               // Actual count returned
	Total       *int      `json:"total,omitempty"`        // Total available (if known)
	RequestID   string    `json:"request_id,omitempty"`   // For tracing
	GeneratedAt time.Time `json:"generated_at"`           // Response timestamp
	LatencyMS   int64     `json:"latency_ms,omitempty"`   // Server-side latency
}

// ResponseError contains error details
type ResponseError struct {
	Code    string                 `json:"code"`              // ERROR_CODE (e.g., BAD_REQUEST, NOT_FOUND)
	Message string                 `json:"message"`           // Human-readable message
	Field   string                 `json:"field,omitempty"`   // Field that failed validation
	Details map[string]interface{} `json:"details,omitempty"` // Additional context
}

// NewSuccessResponse creates a standard success response
func NewSuccessResponse(data interface{}, meta ResponseMeta) StandardResponse {
	return StandardResponse{
		Data:  data,
		Meta:  meta,
		Error: nil,
	}
}

// NewSuccessResponseWithSummary creates a success response with summary
func NewSuccessResponseWithSummary(data interface{}, summary interface{}, meta ResponseMeta) StandardResponse {
	return StandardResponse{
		Data:    data,
		Summary: summary,
		Meta:    meta,
		Error:   nil,
	}
}

// NewErrorResponse creates a standard error response
func NewErrorResponse(code string, message string, requestID string) StandardResponse {
	return StandardResponse{
		Data: nil,
		Meta: ResponseMeta{
			RequestID:   requestID,
			GeneratedAt: time.Now().UTC(),
		},
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// NewValidationErrorResponse creates a validation error response
func NewValidationErrorResponse(field string, message string, requestID string) StandardResponse {
	return StandardResponse{
		Data: nil,
		Meta: ResponseMeta{
			RequestID:   requestID,
			GeneratedAt: time.Now().UTC(),
		},
		Error: &ResponseError{
			Code:    "VALIDATION_ERROR",
			Message: message,
			Field:   field,
		},
	}
}

// Error codes (constants for consistency)
const (
	ErrorBadRequest     = "BAD_REQUEST"
	ErrorNotFound       = "NOT_FOUND"
	ErrorInternalServer = "INTERNAL_SERVER_ERROR"
	ErrorValidation     = "VALIDATION_ERROR"
	ErrorUnauthorized   = "UNAUTHORIZED"
	ErrorForbidden      = "FORBIDDEN"
	ErrorRateLimit      = "RATE_LIMIT_EXCEEDED"
)

// PaginationParams holds common pagination parameters
type PaginationParams struct {
	Limit  int `form:"limit" binding:"omitempty,min=1,max=1000"`
	Offset int `form:"offset" binding:"omitempty,min=0"`
	Sort   string `form:"sort"`  // Comma-separated list, use - prefix for DESC
}

// GetLimitOrDefault returns limit or default value
func (p PaginationParams) GetLimitOrDefault(defaultVal int) int {
	if p.Limit > 0 {
		return p.Limit
	}
	return defaultVal
}

// GetOffsetOrDefault returns offset or default value
func (p PaginationParams) GetOffsetOrDefault(defaultVal int) int {
	if p.Offset >= 0 {
		return p.Offset
	}
	return defaultVal
}

// DateRangeParams holds common date range parameters
type DateRangeParams struct {
	DateFrom string `form:"date_from" binding:"omitempty,datetime=2006-01-02"`
	DateTo   string `form:"date_to" binding:"omitempty,datetime=2006-01-02"`
}

