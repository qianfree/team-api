package common

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

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
// 优先用驱动类型化断言（pgconn.PgError / lib/pq），PG 升级或错误文案变化不影响判定；
// 字符串兜底仅保留 "duplicate key"（覆盖被 ORM 层重新拼装、丢失类型的错误），
// 不再匹配裸 "23505"。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "duplicate key")
}
