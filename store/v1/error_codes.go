package v1

import "fmt"

const (
	ErrDuplicateConstraintUsername = "username_key"
	ErrDuplicateConstraintEmail    = "email_key"
	HandlerStartTime               = "HANDLER_START_TIME"
	HttpEndpointErrorKey           = "HTTP_ERROR"
)

var (
	ErrMissingUserInContext = fmt.Errorf("ERR_MISSING_USER_IN_CONTEXT")
)
