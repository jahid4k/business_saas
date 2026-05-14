// Package response provides standardised JSON response helpers for the
// BusinessSAAS API. Every endpoint returns one of two shapes:
//
// Success:
//
//	{
//	  "success":    true,
//	  "data":       { ... },
//	  "message":    "OK",
//	  "request_id": "abc123"
//	}
//
// Error:
//
//	{
//	  "success": false,
//	  "error": {
//	    "code":    "INVALID_CREDENTIALS",
//	    "message": "Invalid email or password"
//	  },
//	  "request_id": "abc123"
//	}
//
// Handlers must NEVER construct their own JSON. They must use this package.
package response

import (
	"github.com/gofiber/fiber/v3"
)

// ----------------------------------------------------------
// Response envelope types
// ----------------------------------------------------------

// successResponse is the JSON shape for every successful API response.
type successResponse struct {
	Success   bool   `json:"success"`
	Data      any    `json:"data,omitempty"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// errorBody is the nested error object inside an error response.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorResponse is the JSON shape for every failed API response.
type errorResponse struct {
	Success   bool      `json:"success"`
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

// ----------------------------------------------------------
// Success helpers
// ----------------------------------------------------------

// OK sends a 200 response with optional data and a message.
//
// Usage:
//
//	return response.OK(c, fiber.Map{"user": u}, "User fetched")
func OK(c fiber.Ctx, data any, message string) error {
	return c.Status(fiber.StatusOK).JSON(successResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		RequestID: requestID(c),
	})
}

// Created sends a 201 response with optional data and a message.
//
// Usage:
//
//	return response.Created(c, fiber.Map{"user": u}, "Account created")
func Created(c fiber.Ctx, data any, message string) error {
	return c.Status(fiber.StatusCreated).JSON(successResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		RequestID: requestID(c),
	})
}

// NoContent sends a 204 response with no body.
//
// Usage:
//
//	return response.NoContent(c)
func NoContent(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// ----------------------------------------------------------
// Error helpers
// ----------------------------------------------------------

// BadRequest sends a 400 response.
func BadRequest(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusBadRequest, code, message)
}

// Unauthorized sends a 401 response.
func Unauthorized(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusUnauthorized, code, message)
}

// Forbidden sends a 403 response.
func Forbidden(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusForbidden, code, message)
}

// NotFound sends a 404 response.
func NotFound(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusNotFound, code, message)
}

// Conflict sends a 409 response.
func Conflict(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusConflict, code, message)
}

// UnprocessableEntity sends a 422 response.
func UnprocessableEntity(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusUnprocessableEntity, code, message)
}

// TooManyRequests sends a 429 response.
func TooManyRequests(c fiber.Ctx, code, message string) error {
	return sendError(c, fiber.StatusTooManyRequests, code, message)
}

// InternalServerError sends a 500 response.
// The message shown to the client is always generic.
// Log the real error server-side before calling this.
func InternalServerError(c fiber.Ctx) error {
	return sendError(c,
		fiber.StatusInternalServerError,
		"INTERNAL_SERVER_ERROR",
		"An unexpected error occurred. Please try again later.",
	)
}

// NotImplemented sends a 501 response.
// Used for stubbed routes during Phase 1 development.
func NotImplemented(c fiber.Ctx) error {
	return sendError(c,
		fiber.StatusNotImplemented,
		"NOT_IMPLEMENTED",
		"This endpoint is not yet implemented.",
	)
}

// ----------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------

// sendError is the internal method that writes all error responses.
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

// requestID extracts the request ID set by the requestid middleware.
// Returns an empty string if not present (safe — field is omitempty).
func requestID(c fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return ""
}
