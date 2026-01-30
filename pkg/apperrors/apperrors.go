package apperrors

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Define ErrorCode type
type ErrorCode string

const (
	// General Errors
	ErrCodeInternalServer    ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeResourceNotFound  ErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeBadRequest        ErrorCode = "MALFORMED_REQUEST"
	ErrCodeFailedValidation  ErrorCode = "FAILED_VALIDATION"

	// User Errors
	ErrCodeUserNotFound          ErrorCode = "USER_NOT_FOUND"
	ErrCodeEmailAlreadyExists    ErrorCode = "EMAIL_ALREADY_EXISTS"
	ErrCodeUsernameAlreadyExists ErrorCode = "USERNAME_ALREADY_EXISTS"
	ErrCodeInvalidEmail          ErrorCode = "INVALID_EMAIL"
	ErrCodeInvalidPassword       ErrorCode = "INVALID_PASSWORD"

	// Auth Errors
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrCodeInvalidToken       ErrorCode = "INVALID_TOKEN"
	ErrCodeUnauthenticated    ErrorCode = "UNAUTHENTICATED"
	ErrCodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
	ErrCodeResourceForbidden  ErrorCode = "RESOURCE_FORBIDDEN"
	ErrCodeSignOutRequired    ErrorCode = "SIGN_OUT_REQUIRED"

	// DB Errors
	ErrCodeEditConflict ErrorCode = "EDIT_CONFLICT"

	// Media Errors
	ErrCodeFileTooBig        ErrorCode = "FILE_TOO_BIG"
	ErrCodeUnsupportedFormat ErrorCode = "UNSUPPORTED_FORMAT"
	ErrCodeProcessingFailed  ErrorCode = "PROCESSING_FAILED"
)

// Readable error messages tied to ErrorCodes
var ErrorMessages = map[ErrorCode]string{
	// General
	ErrCodeInternalServer:    "something went wrong, please try again later",
	ErrCodeRateLimitExceeded: "too many requests sent in too short a timeframe",
	ErrCodeResourceNotFound:  "could not locate the requested resource",
	ErrCodeBadRequest:        "request was malformed and couldn't be processed",
	ErrCodeFailedValidation:  "request contained invalid data",
	// User
	ErrCodeUserNotFound:          "user not found",
	ErrCodeEmailAlreadyExists:    "an account with this email already exists",
	ErrCodeUsernameAlreadyExists: "an account with this username already exists",
	ErrCodeInvalidEmail:          "please enter a valid email address",
	ErrCodeInvalidPassword:       "please enter a valid password",
	// Auth
	ErrCodeInvalidCredentials: "invalid authentication credentials",
	ErrCodeTokenExpired:       "your session has expired, please log in again",
	ErrCodeInvalidToken:       "an invalid token was provided",
	ErrCodeUnauthenticated:    "you must be signed in before attempting to access this resource",
	ErrCodePermissionDenied:   "you do not have permission to perform this action",
	ErrCodeResourceForbidden:  "you do not have access to this resource",
	ErrCodeSignOutRequired:    "you cannot access this resource while you are signed in",
	// DB
	ErrCodeEditConflict: "another process is already editing this item",
	// Media
	ErrCodeFileTooBig:        "this file is too large to be processed",
	ErrCodeUnsupportedFormat: "this file format is not supported",
	ErrCodeProcessingFailed:  "failed to process your file, please try again",
}

// Match ErrorCode to corresponding HTTP Status Code - return StatusInternalServerError if not match is found
func getHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrCodeResourceNotFound, ErrCodeUserNotFound:
		return http.StatusNotFound
	case ErrCodeBadRequest:
		return http.StatusBadRequest
	case ErrCodeFailedValidation, ErrCodeInvalidEmail, ErrCodeInvalidPassword, ErrCodeFileTooBig, ErrCodeInvalidToken:
		return http.StatusUnprocessableEntity
	case ErrCodeUnsupportedFormat:
		return http.StatusUnsupportedMediaType
	case ErrCodeEmailAlreadyExists, ErrCodeUsernameAlreadyExists, ErrCodeEditConflict:
		return http.StatusConflict
	case ErrCodeInvalidCredentials, ErrCodeTokenExpired, ErrCodeUnauthenticated:
		return http.StatusUnauthorized
	case ErrCodePermissionDenied, ErrCodeSignOutRequired:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

type AppError struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Err       error          `json:"error,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

func NewAppError(code ErrorCode, err error, details map[string]any, reqID string) AppError {
	return AppError{
		Code:      code,
		Message:   ErrorMessages[code],
		Err:       err,
		Details:   details,
		RequestID: reqID,
	}
}

func (e AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s]: %s", e.Message, e.Err)
	}
	return e.Message
}

// type FieldError struct {
// 	Field   string `json:"field"`
// 	Message string `json:"message"`
// }

type APIError struct {
	Success    bool     `json:"success"`
	StatusCode int      `json:"status_code"`
	Error      AppError `json:"error"`
	Redirect   string   `json:"redirect,omitempty"`
	RequestID  string   `json:"request_id,omitempty"`
}

func (e AppError) ToAPIError(redirect string) APIError {
	apiErr := APIError{
		Success:    false,
		StatusCode: getHTTPStatus(e.Code),
		Error:      e,
		Redirect:   redirect,
		RequestID:  e.RequestID,
	}

	return apiErr
}

type HTTPError struct {
	StatusCode int            `json:"-"`
	Error      string         `json:"error"`
	Message    string         `json:"message"`
	Code       ErrorCode      `json:"code"`
	Details    map[string]any `json:"details,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
}

// Convert AppError to HTTPError for use in web handlers
func (e AppError) ToHTTPError() HTTPError {
	httpErr := HTTPError{
		StatusCode: http.StatusInternalServerError,
		Error:      "Internal Server Error",
		Message:    ErrorMessages[ErrCodeInternalServer],
		Code:       ErrCodeInternalServer,
		RequestID:  e.RequestID,
	}

	statusCode := getHTTPStatus(e.Code)
	message := ErrorMessages[e.Code]
	// In case ErrorMessages map doesn't contain this code
	if message == "" {
		message = e.Message
	}

	httpErr.StatusCode = statusCode
	httpErr.Error = http.StatusText(statusCode)
	httpErr.Message = message
	httpErr.Code = e.Code

	// Attach information fields if they exist
	if e.Details != nil || len(e.Details) > 0 {
		httpErr.Details = e.Details
	}

	return httpErr
}

// Convert AppError to simple string for consumption in toast/notification components
func (e AppError) ToFlash() string {
	return e.Message
}

// Convert AppError to APIError, and send it as an echo.JSON response
func SendAPIErrorResponse(c echo.Context, err AppError, redirect string) error {
	apiErr := err.ToAPIError(redirect)

	return c.JSON(getHTTPStatus(err.Code), apiErr)
}
