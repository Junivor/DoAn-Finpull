package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// DataResponse writes API response with status and data.
func DataResponse(c echo.Context, statusCode int, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{
		Status:  statusCode,
		Message: http.StatusText(statusCode),
		Data:    data,
	})
}

// ListResponse writes paginated list response.
func ListResponse(c echo.Context, rows interface{}, total int64) error {
	return DataResponse(c, http.StatusOK, &ListDataResponse{
		Rows:  rows,
		Total: total,
	})
}

// SuccessResponse writes success response.
func SuccessResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusOK, data)
}

// CreatedResponse writes created response.
func CreatedResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusCreated, data)
}

// NoContentResponse writes no content response.
func NoContentResponse(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// BadRequestResponse writes bad request error.
func BadRequestResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusBadRequest, data)
}

// UnauthorizedResponse writes unauthorized error.
func UnauthorizedResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusUnauthorized, data)
}

// ForbiddenResponse writes forbidden error.
func ForbiddenResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusForbidden, data)
}

// NotFoundResponse writes not found error.
func NotFoundResponse(c echo.Context, data interface{}) error {
	return DataResponse(c, http.StatusNotFound, data)
}

// InternalServerErrorResponse writes internal server error.
func InternalServerErrorResponse(c echo.Context) error {
	return DataResponse(c, http.StatusInternalServerError, "Something went wrong")
}

// AppErrorResponse writes application error response.
func AppErrorResponse(c echo.Context, err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return DataResponse(c, appErr.Status, []*AppError{appErr})
	}
	return InternalServerErrorResponse(c)
}
