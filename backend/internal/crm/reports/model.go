// backend/internal/crm/reports/model.go
package reports

// CRMSummary is the aggregate overview of all CRM data for an org.
type CRMSummary struct {
	TotalContacts  int     `json:"total_contacts"`
	TotalCompanies int     `json:"total_companies"`
	TotalLeads     int     `json:"total_leads"`
	TotalDeals     int     `json:"total_deals"`
	OpenDeals      int     `json:"open_deals"`
	WonDeals       int     `json:"won_deals"`
	LostDeals      int     `json:"lost_deals"`
	TotalDealValue float64 `json:"total_deal_value"`
	WonDealValue   float64 `json:"won_deal_value"`
}

// OverdueTask wraps a task reference with how many days overdue it is.
type OverdueTask struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	DaysOverdue int    `json:"days_overdue"`
}

// ActivityStat groups activity count by type.
type ActivityStat struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// AgendaItem represents a unified item in the Today's Agenda view.
type AgendaItem struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"` // "task" or "activity"
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	DueDate     *string `json:"due_date,omitempty"`    // for tasks
	OccurredAt  *string `json:"occurred_at,omitempty"` // for activities
	RelatedType string  `json:"related_type,omitempty"`
	RelatedID   string  `json:"related_id,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

// Agenda is the response for the Smart Task Dashboard view.
type Agenda struct {
	Items []AgendaItem `json:"items"`
}

// RepPerformance represents the activity and deal metrics for a single sales rep.
type RepPerformance struct {
	RepID       string  `json:"rep_id"`
	RepName     string  `json:"rep_name"`
	Calls       int     `json:"calls"`
	Meetings    int     `json:"meetings"`
	DealsClosed int     `json:"deals_closed"`
	RevenueWon  float64 `json:"revenue_won"`
}

// Forecast represents the revenue forecast based on deal stages.
type Forecast struct {
	TotalPipelineValue float64 `json:"total_pipeline_value"`
	WeightedForecast   float64 `json:"weighted_forecast"`
}
