package scheduler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListJobs(c fiber.Ctx) error {
	res, err := h.service.ListJobs(c.Context())
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "Jobs retrieved successfully")
}

func (h *Handler) ListJobRuns(c fiber.Ctx) error {
	jobName := c.Params("name")
	
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	res, err := h.service.ListJobRuns(c.Context(), jobName, limit, offset)
	if err != nil {
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "Job runs retrieved successfully")
}

func (h *Handler) TriggerJob(c fiber.Ctx) error {
	jobName := c.Params("name")
	
	err := h.service.Trigger(c.Context(), jobName)
	if err != nil {
		return response.InternalServerError(c)
	}
	
	return response.OK(c, nil, "Job triggered successfully")
}
