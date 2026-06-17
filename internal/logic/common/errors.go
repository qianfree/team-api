package common

import (
	"database/sql"
	"errors"
	"strings"

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

// IgnoreScanNoRows normalizes sql.ErrNoRows to nil.
// GoFrame's Scan(&struct) returns sql.ErrNoRows when no record is found,
// but callers typically check the zero-value result afterwards.
func IgnoreScanNoRows(err error) error {
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// IsDuplicateKeyError reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505). Used to convert race-condition insert failures
// into friendly business errors at the data layer's last line of defense.
//
// Uses string matching on the driver error message. Switch to a typed assertion
// (pq.Error / pgconn.PgError) if a Postgres driver becomes a direct dependency.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key")
}
