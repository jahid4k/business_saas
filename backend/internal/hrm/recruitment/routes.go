// backend/internal/hrm/recruitment/routes.go
package recruitment

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM recruitment route. All routes are
// authenticated HRM routes — Phase 4A is internal-only, no public careers
// page (see the phase plan). Every route below carries a permFn("hrm....")
// argument: TestPermissions_AllRoutesProtected (internal/tests/unit/
// architecture) walks internal/hrm/**/routes.go and hard-fails any
// registration without one, and its openRoutesAllowlist is empty — which is
// also precisely why a public route could not live in this package even if
// one were wanted here.
//
//	Pipelines     GET/POST         /organizations/:orgId/hrm/recruitment/pipelines
//	              GET/PATCH/DELETE .../pipelines/:pipelineId
//	              GET/POST         .../pipelines/:pipelineId/stages
//	              POST             .../pipelines/:pipelineId/stages/reorder   (before /:stageId)
//	              PATCH/DELETE     .../pipelines/:pipelineId/stages/:stageId
//	Requisitions  GET/POST         .../requisitions
//	              GET/PATCH        .../requisitions/:requisitionId
//	              POST             .../requisitions/:requisitionId/{submit,close}
//	Postings      GET/POST         .../postings
//	              GET/PATCH/DELETE .../postings/:postingId
//	              POST             .../postings/:postingId/{publish,close}
//	Candidates    GET/POST         .../candidates
//	              GET/PATCH/DELETE .../candidates/:candidateId
//	              POST             .../candidates/:candidateId/resume        (multipart, hrm.candidates.manage)
//	              GET              .../candidates/:candidateId/resume        (hrm.candidates.download_resume)
//	Applications  GET/POST         .../applications
//	              GET              .../applications/:applicationId
//	              GET              .../applications/:applicationId/history
//	              POST             .../applications/:applicationId/{move,reject,withdraw}
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/recruitment", requireAuth, requireOrgMatch)

	// ── Pipelines + stages ──────────────────────────────────────────────
	pipelines := base.Group("/pipelines")
	pipelines.Get("", permFn("hrm.recruitment.view"), handler.ListPipelines)
	pipelines.Post("", permFn("hrm.recruitment.manage"), handler.CreatePipeline)
	pipelines.Get("/:pipelineId", permFn("hrm.recruitment.view"), handler.GetPipeline)
	pipelines.Patch("/:pipelineId", permFn("hrm.recruitment.manage"), handler.UpdatePipeline)
	pipelines.Delete("/:pipelineId", permFn("hrm.recruitment.manage"), handler.DeletePipeline)

	pipelines.Get("/:pipelineId/stages", permFn("hrm.recruitment.view"), handler.ListStages)
	pipelines.Post("/:pipelineId/stages", permFn("hrm.recruitment.manage"), handler.CreateStage)
	// /reorder must register before /:stageId — a literal segment loses to
	// a param only if registered after it (the /companies/enrich precedent).
	pipelines.Post("/:pipelineId/stages/reorder", permFn("hrm.recruitment.manage"), handler.ReorderStages)
	pipelines.Patch("/:pipelineId/stages/:stageId", permFn("hrm.recruitment.manage"), handler.UpdateStage)
	pipelines.Delete("/:pipelineId/stages/:stageId", permFn("hrm.recruitment.manage"), handler.DeleteStage)

	// ── Requisitions ────────────────────────────────────────────────────
	requisitions := base.Group("/requisitions")
	requisitions.Get("", permFn("hrm.recruitment.view"), handler.ListRequisitions)
	requisitions.Post("", permFn("hrm.recruitment.manage"), handler.CreateRequisition)
	requisitions.Get("/:requisitionId", permFn("hrm.recruitment.view"), handler.GetRequisition)
	requisitions.Patch("/:requisitionId", permFn("hrm.recruitment.manage"), handler.UpdateRequisition)
	requisitions.Post("/:requisitionId/submit", permFn("hrm.recruitment.manage"), handler.SubmitRequisition)
	requisitions.Post("/:requisitionId/close", permFn("hrm.recruitment.manage"), handler.CloseRequisition)

	// ── Postings ────────────────────────────────────────────────────────
	postings := base.Group("/postings")
	postings.Get("", permFn("hrm.recruitment.view"), handler.ListPostings)
	postings.Post("", permFn("hrm.recruitment.manage"), handler.CreatePosting)
	postings.Get("/:postingId", permFn("hrm.recruitment.view"), handler.GetPosting)
	postings.Patch("/:postingId", permFn("hrm.recruitment.manage"), handler.UpdatePosting)
	postings.Delete("/:postingId", permFn("hrm.recruitment.manage"), handler.DeletePosting)
	postings.Post("/:postingId/publish", permFn("hrm.recruitment.manage"), handler.PublishPosting)
	postings.Post("/:postingId/close", permFn("hrm.recruitment.manage"), handler.ClosePosting)

	// ── Candidates ──────────────────────────────────────────────────────
	candidates := base.Group("/candidates")
	candidates.Get("", permFn("hrm.candidates.view"), handler.ListCandidates)
	candidates.Post("", permFn("hrm.candidates.manage"), handler.CreateCandidate)
	candidates.Get("/:candidateId", permFn("hrm.candidates.view"), handler.GetCandidate)
	candidates.Patch("/:candidateId", permFn("hrm.candidates.manage"), handler.UpdateCandidate)
	candidates.Delete("/:candidateId", permFn("hrm.candidates.manage"), handler.DeleteCandidate)
	candidates.Post("/:candidateId/resume", permFn("hrm.candidates.manage"), handler.UploadResume)
	// download_resume is its own permission — the sharpest data in the
	// module gets its own gate, the hrm.leave.adjust_balance precedent.
	candidates.Get("/:candidateId/resume", permFn("hrm.candidates.download_resume"), handler.DownloadResume)

	// ── Applications ────────────────────────────────────────────────────
	applications := base.Group("/applications")
	applications.Get("", permFn("hrm.candidates.view"), handler.ListApplications)
	applications.Post("", permFn("hrm.candidates.manage"), handler.CreateApplication)
	applications.Get("/:applicationId", permFn("hrm.candidates.view"), handler.GetApplication)
	applications.Get("/:applicationId/history", permFn("hrm.candidates.view"), handler.GetApplicationHistory)
	applications.Post("/:applicationId/move", permFn("hrm.candidates.manage"), handler.MoveApplication)
	applications.Post("/:applicationId/reject", permFn("hrm.candidates.manage"), handler.RejectApplication)
	applications.Post("/:applicationId/withdraw", permFn("hrm.candidates.manage"), handler.WithdrawApplication)
}
