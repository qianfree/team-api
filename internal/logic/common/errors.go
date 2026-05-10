package common

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/consts"
)

// NewBusinessError creates a business error with code and message.
func NewBusinessError(code int, message string) error {
	return gerror.NewCode(gcode.New(code, message, nil), message)
}

// NewNotFoundError creates a 404 Not Found error.
func NewNotFoundError(resource string) error {
	return gerror.NewCode(
		gcode.New(consts.CodeNotFound, resource+"不存在", nil),
		resource+"不存在",
	)
}

// NewForbiddenError creates a 403 Forbidden error.
func NewForbiddenError(message string) error {
	return gerror.NewCode(
		gcode.New(consts.CodeForbidden, message, nil),
		message,
	)
}

// NewBadRequestError creates a 400 Bad Request error.
func NewBadRequestError(message string) error {
	return gerror.NewCode(
		gcode.New(consts.CodeBadRequest, message, nil),
		message,
	)
}

// NewUnauthorizedError creates a 401 Unauthorized error.
func NewUnauthorizedError(message string) error {
	return gerror.NewCode(
		gcode.New(consts.CodeUnauthorized, message, nil),
		message,
	)
}
