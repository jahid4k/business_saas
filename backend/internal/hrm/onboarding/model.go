// backend/internal/hrm/onboarding/model.go
package onboarding

import (
	"errors"
	"time"
)

// subjectRef is the raw hrm_employees lookup row used to build a
// checklists.SubjectContext. Unexported — this package's only external
// surface is Service; the checklist engine never sees an hrm_employees row,
// only the SubjectContext derived from it.
type subjectRef struct {
	EmployeeID     string
	UserID         *string // nil when the employee has no platform account
	ManagerUserID  *string // nil when absent, manager row deleted, or manager has no platform account (one LEFT JOIN collapses all three)
	HireDate       time.Time
	DisplayName    string
	EmployeeNumber string
}

var (
	ErrEmployeeNotFound  = errors.New("employee not found")
	ErrNoDefaultTemplate = errors.New("no default onboarding checklist template is configured for this organization")
)
