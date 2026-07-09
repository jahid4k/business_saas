// backend/internal/hrm/reports/model.go
package reports

// HRMSummary aggregates key HRM counts in a single dashboard widget.
type HRMSummary struct {
	TotalEmployees       int64 `json:"total_employees"`
	ActiveEmployees      int64 `json:"active_employees"`
	OnLeaveEmployees     int64 `json:"on_leave_employees"`
	TerminatedEmployees  int64 `json:"terminated_employees"`
	TotalDepartments     int64 `json:"total_departments"`
	TotalPositions       int64 `json:"total_positions"`
	PendingLeaveRequests int64 `json:"pending_leave_requests"`
	ApprovedLeaveToday   int64 `json:"approved_leave_today"`
}

// HeadcountByDepartment breaks down active employee counts per department.
type HeadcountByDepartment struct {
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name"`
	Headcount      int64  `json:"headcount"`
}

// LeaveSummaryByType shows leave request statistics grouped by leave type.
type LeaveSummaryByType struct {
	LeaveTypeID   string  `json:"leave_type_id"`
	LeaveTypeName string  `json:"leave_type_name"`
	TotalRequests int64   `json:"total_requests"`
	Approved      int64   `json:"approved"`
	Pending       int64   `json:"pending"`
	Rejected      int64   `json:"rejected"`
	TotalDays     float64 `json:"total_days"` // sum of approved total_days
}
