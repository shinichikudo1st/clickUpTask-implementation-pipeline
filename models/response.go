package models

// SuccessResponse is the standard ApexSuite success envelope.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

// ErrorDetail is the structured error payload.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the standard ApexSuite error envelope.
type ErrorResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   ErrorDetail `json:"error"`
}
