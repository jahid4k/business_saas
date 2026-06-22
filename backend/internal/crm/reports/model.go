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
