package http

import "time"

// APIResponse represents standard API response.
type APIResponse struct {
	Status  int         `json:"status" example:"200"`
	Message string      `json:"message" example:"OK"`
	Data    interface{} `json:"data,omitempty"`
}

// APIResponse400Err represents 400 error response.
type APIResponse400Err struct {
	Status  int               `json:"status" example:"400"`
	Message string            `json:"message" example:"Bad Request"`
	Data    []ValidationError `json:"data,omitempty"`
}

// APIResponse401Err represents 401 error response.
type APIResponse401Err struct {
	Status  int    `json:"status" example:"401"`
	Message string `json:"message" example:"Unauthorized"`
	Data    string `json:"data,omitempty" example:"Token is invalid"`
}

// APIResponse500Err represents 500 error response.
type APIResponse500Err struct {
	Status  int    `json:"status" example:"500"`
	Message string `json:"message" example:"Internal Server Error"`
	Data    string `json:"data,omitempty"`
}

// ValidationError represents validation error detail.
type ValidationError struct {
	Code    string                 `json:"code,omitempty" example:"ERR_REQUIRED"`
	Field   string                 `json:"field,omitempty" example:"name"`
	Message string                 `json:"message,omitempty" example:"Name is required"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// ListDataResponse represents paginated list response.
type ListDataResponse struct {
	Rows  interface{} `json:"rows"`
	Total int64       `json:"total"`
}

// TimeRange represents time range filter.
type TimeRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}
