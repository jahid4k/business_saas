// backend/pkg/response/response.go
package response

import "github.com/gofiber/fiber/v3"

// ----------------------------------------------------------
// Response envelope types
// ----------------------------------------------------------

type successResponse struct {
	Success   bool   `json:"success"`
	Data      any    `json:"data,omitempty"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Success   bool      `json:"success"`
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

// ----------------------------------------------------------
// Success helpers
// ----------------------------------------------------------

func OK(c fiber.Ctx, data any, message string) error {
	return c.Status(fiber.StatusOK).JSON(successResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		RequestID: requestID(c),
	})
}

func Created(c fiber.Ctx, data any, message string) error {
	return c.Status(fiber.StatusCreated).JSON(successResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		RequestID: requestID(c),
	})
}

func NoContent(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// ----------------------------------------------------------
// Error helpers
// ----------------------------------------------------------

func BadRequest(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusBadRequest, code, message)
}

func Unauthorized(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusUnauthorized, code, message)
}

func Forbidden(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusForbidden, code, message)
}

func NotFound(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusNotFound, code, message)
}

func Conflict(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusConflict, code, message)
}

func TooManyRequests(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusTooManyRequests, code, message)
}

func InternalServerError(c fiber.Ctx) error {
	return sendError(c, fiber.StatusInternalServerError,
		"INTERNAL_SERVER_ERROR",
		"An unexpected error occurred. Please try again later.",
	)
}

func NotImplemented(c fiber.Ctx) error {
	return sendError(c, fiber.StatusNotImplemented,
		"NOT_IMPLEMENTED",
		"This feature is not yet available.",
	)
}

// ----------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------

func sendError(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(errorResponse{
		Success: false,
		Error: errorBody{
			Code:    code,
			Message: message,
		},
		RequestID: requestID(c),
	})
}

func requestID(c fiber.Ctx) string {
	id, _ := c.Locals("request_id").(string)
	return id
}
