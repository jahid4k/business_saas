package dashboard

import "time"

type KPIs struct {
	ActivePipelineValue float64 `json:"active_pipeline_value"`
	TotalHeadcount      int     `json:"total_headcount"`
	PendingApprovals    int     `json:"pending_approvals"`
}

type ActionItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // e.g., "stagnant_deal", "pending_approval"
	ActionURL   string    `json:"action_url"`
}

type DashboardResponse struct {
	KPIs        KPIs          `json:"kpis"`
	ActionItems []*ActionItem `json:"action_items"`
}
