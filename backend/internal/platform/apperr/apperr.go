// Package apperr holds the error values shared by all modules. Services return
// them; httpx maps them to HTTP responses (spec §8.1).
package apperr

import "net/http"

type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Is lets errors.Is match on the error code, so With() copies still match the sentinel.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// With returns a copy with a different message.
func (e *Error) With(msg string) *Error {
	c := *e
	c.Message = msg
	return &c
}

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

var (
	ErrUnauthenticated        = New(http.StatusUnauthorized, "UNAUTHENTICATED", "Please sign in")
	ErrForbidden              = New(http.StatusForbidden, "FORBIDDEN", "You are not allowed to do this")
	ErrPasswordChangeRequired = New(http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "You must change your password first")
	ErrMobileNotAllowed       = New(http.StatusForbidden, "MOBILE_NOT_ALLOWED", "Security staff use the web app, not the mobile app")
	ErrNotFound               = New(http.StatusNotFound, "NOT_FOUND", "Not found")
	ErrInvalidState           = New(http.StatusConflict, "INVALID_STATE", "This action is not allowed in the current state")
	ErrApprovalWindowClosed   = New(http.StatusConflict, "APPROVAL_WINDOW_CLOSED", "The approval window has closed")
	ErrVisitorAlreadyActive   = New(http.StatusConflict, "VISITOR_ALREADY_ACTIVE", "This visitor already has an active visit")
	ErrDuplicate              = New(http.StatusConflict, "DUPLICATE", "Already exists")
	ErrFileTooLarge           = New(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "File is too large")
	ErrUnsupportedMedia       = New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Only JPEG and PNG images are accepted")
	ErrPhotosRequired         = New(http.StatusUnprocessableEntity, "PHOTOS_REQUIRED", "Visitor photo and ID photo are both required")
	ErrWrongVisitDate         = New(http.StatusUnprocessableEntity, "WRONG_VISIT_DATE", "This visit is not scheduled for today")
	ErrAccountLocked          = New(http.StatusLocked, "ACCOUNT_LOCKED", "Account temporarily locked. Try again later")
	ErrRateLimited            = New(http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
	ErrValidation             = New(http.StatusBadRequest, "VALIDATION_ERROR", "Some fields are invalid")
)

// Validation builds a 400 with per-field messages in details.fields.
func Validation(fields map[string]string) *Error {
	e := *ErrValidation
	f := make(map[string]any, len(fields))
	for k, v := range fields {
		f[k] = v
	}
	e.Details = map[string]any{"fields": f}
	return &e
}

// DuplicateField is a 409 that names the field that clashed.
func DuplicateField(field, msg string) *Error {
	e := *ErrDuplicate
	e.Message = msg
	e.Details = map[string]any{"fields": map[string]any{field: msg}}
	return &e
}
