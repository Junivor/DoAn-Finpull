package http

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ReadAndValidateRequest reads and validates request body.
func ReadAndValidateRequest(c echo.Context, req interface{}) interface{} {
	// Bind request
	if err := c.Bind(req); err != nil {
		return validatorDefaultRules(err)
	}

	// Set default values
	if err := defaults.Set(req); err != nil {
		return validatorDefaultRules(err)
	}

	// Validate struct
	if err := validate.StructCtx(c.Request().Context(), req); err != nil {
		return validatorDefaultRules(err)
	}

	return nil
}

func validatorDefaultRules(err error) interface{} {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		errs := make([]ValidationError, 0, len(validationErrors))
		for _, e := range validationErrors {
			code := "ERR_" + strings.ToUpper(e.Tag())
			errs = append(errs, ValidationError{
				Code:    code,
				Field:   e.Field(),
				Message: getErrorMessage(e),
				Params:  getErrorParams(e),
			})
		}
		return errs
	}

	var he *echo.HTTPError
	if errors.As(err, &he) {
		return []ValidationError{{
			Code:    "ERR_UNKNOWN",
			Message: fmt.Sprintf("%v", he.Message),
		}}
	}

	return []ValidationError{{
		Code:    "ERR_UNKNOWN",
		Message: err.Error(),
	}}
}

func getErrorMessage(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "max":
		if fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, strings.ReplaceAll(fe.Param(), " ", ", "))
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s failed validation: %s", field, fe.Tag())
	}
}

func getErrorParams(fe validator.FieldError) map[string]interface{} {
	params := make(map[string]interface{})

	switch fe.Tag() {
	case "min", "gte":
		params["min"] = fe.Param()
	case "max", "lte":
		params["max"] = fe.Param()
	case "gt", "lt":
		params["value"] = fe.Param()
	case "oneof":
		params["options"] = strings.Split(fe.Param(), " ")
	}

	return params
}
