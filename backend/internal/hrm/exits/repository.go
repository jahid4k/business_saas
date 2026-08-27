// backend/internal/hrm/exits/repository.go
package exits

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// Repository is the data access interface for exit management.
//
// TENANT ISOLATION: every method takes orgID. Clearance items have no org_id
// of their own and are reached by JOINing through hrm_exits — the
// hrm_approval_decisions / platform_ticket_comments precedent.
//
// ⚠ There is deliberately no FindClearanceCompletion or similar. Completion
// is derived from the items in the service, and adding a repository method
// that answers it would be one short step from caching the answer in a
// column, which migration 00114 exists to avoid.
type Repository interface {
	// Exits
	CreateExit(ctx context.Context, e *Exit) error
	FindExitByRef(ctx context.Context, orgID, ref string) (*Exit, error)
	FindOpenExitForEmployee(ctx context.Context, orgID, employeeID string) (*Exit, error)
	FindExits(ctx context.Context, orgID string, f ListFilter) ([]*Exit, error)
	CountExits(ctx context.Context, orgID string, f ListFilter) (int, error)
	UpdateExit(ctx context.Context, e *Exit) error

	// Source validation. FK-free by design (see migration 00114), so the
	// service checks the referenced decision exists and belongs to the same
	// employee rather than relying on the database to.
	SourceExists(ctx context.Context, orgID string, sourceType SourceType, sourceID, employeeID string) (bool, error)
	// FindSourceNotice returns the notice dates the decision record already
	// holds, so the exit snapshots them rather than inventing its own.
	// Returns nil when the source has none (a termination, which carries no
	// notice period).
	FindSourceNotice(ctx context.Context, orgID string, sourceType SourceType, sourceID string) (*SourceNotice, error)
	// FindTerminationRehireFlag reads hrm_terminations.is_rehire_eligible —
	// the column that has existed since 00034 with no reader.
	FindTerminationRehireFlag(ctx context.Context, orgID, terminationID string) (*bool, error)

	// Clearance
	CreateClearanceItem(ctx context.Context, orgID string, item *ClearanceItem) error
	FindClearanceItems(ctx context.Context, orgID, exitID string) ([]*ClearanceItem, error)
	FindClearanceItemByRef(ctx context.Context, orgID, exitID, ref string) (*ClearanceItem, error)
	UpdateClearanceItem(ctx context.Context, orgID string, item *ClearanceItem) error

	// Settlement (9B)
	// FindExitByFnFRun resolves which exit a run_type='fnf' payroll run
	// settles. The link is hrm_exits.fnf_payslip_run_id, set when the run is
	// attached — payslips never queries hrm_exits itself.
	FindExitByFnFRun(ctx context.Context, orgID, runID string) (*Exit, error)
	// FindExitByFnFRunAnyOrg is the same lookup without an org filter, for
	// the MarkSettled callback: payslips hands back a run id and nothing
	// else, and threading an orgID through that interface would let the two
	// disagree about which tenant a run belongs to. A payslip run id is
	// already a server-side value the caller never supplies, so this widens
	// no boundary a request could cross.
	FindExitByFnFRunAnyOrg(ctx context.Context, runID string) (*Exit, error)
	// FnFRunCurrency reads the currency the settlement will actually be paid
	// in, which is what an advance's own currency has to match. A legitimate
	// read of this table's own FK target (hrm_exits.fnf_payslip_run_id), not
	// a reach into payroll's internals.
	FnFRunCurrency(ctx context.Context, orgID, runID string) (string, error)
	// GratuityRuleAsOf returns the rule in force on asOf: the row with the
	// LATEST effective_date not after it. The 7D SlabsAsOf shape — a rule
	// revised next month must not alter a settlement computed this month.
	GratuityRuleAsOf(ctx context.Context, orgID string, asOf time.Time) (*GratuityRuleRow, error)
	CreateGratuityRule(ctx context.Context, r *GratuityRuleRow) error
	ListGratuityRules(ctx context.Context, orgID string) ([]*GratuityRuleRow, error)
	// EmployeeTenure returns hire date and monthly basic for the gratuity and
	// daily-rate calculations.
	EmployeeTenure(ctx context.Context, orgID, employeeID string) (*EmployeeTenure, error)
	// InsertSettlementLines records the audit trail. Append-only.
	InsertSettlementLines(ctx context.Context, exitID string, lines []SettlementLineRow) error
	FindSettlementLines(ctx context.Context, orgID, exitID string) ([]*SettlementLineRow, error)
	// LinkSettlementLineToPayslip stamps which payslip line a settlement line
	// became, once the payslip exists.
	LinkSettlementLineToPayslip(ctx context.Context, exitID, sourceType, sourceID, payslipLineID string) error

	// Rehire eligibility
	UpsertRehireEligibility(ctx context.Context, r *RehireEligibility) error
	FindRehireEligibility(ctx context.Context, orgID, employeeID string) (*RehireEligibility, error)
	// FindRehireEligibilityByEmail backs recruitment's candidate-create
	// check. Email is the only handle available — a candidate is not yet an
	// employee, so there is no id to join on.
	FindRehireEligibilityByEmail(ctx context.Context, orgID, email string) (*RehireEligibility, error)

	// Exit interviews (9C)
	CreateInterview(ctx context.Context, i *ExitInterview) error
	FindInterviewByExit(ctx context.Context, orgID, exitID string) (*ExitInterview, error)
	FindInterviewByRef(ctx context.Context, orgID, ref string) (*ExitInterview, error)
	UpdateInterview(ctx context.Context, i *ExitInterview) error
	// FindDueInterviews backs the send sweep: scheduled interviews whose date
	// has arrived. Instance-wide (no orgID) because the scheduler runs once
	// for the whole deployment, the benefits.activate_pending_enrollments
	// shape.
	FindDueInterviews(ctx context.Context, asOf time.Time, limit int) ([]*ExitInterview, error)

	// Access revocation (9C)
	// FindExitsDueForRevocation backs the revocation sweep — exits whose last
	// working date has passed and whose access has not yet been cut.
	// Instance-wide, same reason as above.
	FindExitsDueForRevocation(ctx context.Context, asOf time.Time, limit int) ([]*Exit, error)
	// FindEmployeeUserID resolves the platform account to suspend. Returns ""
	// when the employee has none, which is normal and not an error.
	FindEmployeeUserID(ctx context.Context, orgID, employeeID string) (string, error)

	// FindSubject supplies what the checklist engine needs to instantiate an
	// offboarding checklist for this employee. Mirrors onboarding's
	// FindSubject; the anchor date differs (last working date, not hire
	// date), so the caller supplies it rather than this returning it.
	FindSubject(ctx context.Context, orgID, employeeRef string) (*SubjectRef, error)

	// FindEmployeeIDByUserID resolves the caller's own employee record, so a
	// member holding only view_own can find their exit. On this package's OWN
	// repository, the 7D benefits.FindEmployeeIDByUserID precedent — not a
	// cross-package call into employees.Service, which has no such method.
	FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error)
}

// GratuityRuleRow mirrors hrm_gratuity_rules. Converted to the pure
// GratuityRule value type before any arithmetic touches it.
type GratuityRuleRow struct {
	ID                  string          `db:"id"                     json:"id"`
	PublicID            string          `db:"public_id"               json:"public_id"`
	OrgID               string          `db:"org_id"                  json:"org_id"`
	Name                string          `db:"name"                    json:"name"`
	MinYearsOfService   decimal.Decimal `db:"min_years_of_service"    json:"min_years_of_service"`
	DaysPerYear         decimal.Decimal `db:"days_per_year"           json:"days_per_year"`
	BaseComponent       string          `db:"base_component"          json:"base_component"`
	MonthlyDivisor      decimal.Decimal `db:"monthly_divisor"         json:"monthly_divisor"`
	ForfeitOnMisconduct bool            `db:"forfeit_on_misconduct"   json:"forfeit_on_misconduct"`
	EffectiveDate       time.Time       `db:"effective_date"          json:"effective_date"`
	CreatedBy           string          `db:"created_by"              json:"created_by"`
	CreatedAt           time.Time       `db:"created_at"              json:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"              json:"updated_at"`
}

// ToRule converts a stored row into the pure value the arithmetic takes.
func (r *GratuityRuleRow) ToRule() *GratuityRule {
	if r == nil {
		return nil
	}
	return &GratuityRule{
		MinYearsOfService:   r.MinYearsOfService,
		DaysPerYear:         r.DaysPerYear,
		BaseComponent:       r.BaseComponent,
		MonthlyDivisor:      r.MonthlyDivisor,
		ForfeitOnMisconduct: r.ForfeitOnMisconduct,
	}
}

// EmployeeTenure is what gratuity and the daily rate need about a leaver.
type EmployeeTenure struct {
	HireDate   time.Time
	MonthlyPay decimal.Decimal
	IsForCause bool
}

// SettlementLineRow mirrors hrm_exit_settlement_lines. Amount is ALWAYS
// positive; direction is IsCredit.
type SettlementLineRow struct {
	ID            string          `db:"id"                json:"id"`
	PublicID      string          `db:"public_id"          json:"public_id"`
	ExitID        string          `db:"exit_id"            json:"exit_id"`
	SourceType    string          `db:"source_type"        json:"source_type"`
	SourceID      *string         `db:"source_id"          json:"source_id,omitempty"`
	Description   string          `db:"description"        json:"description"`
	Amount        decimal.Decimal `db:"amount"             json:"amount"`
	IsCredit      bool            `db:"is_credit"          json:"is_credit"`
	Currency      string          `db:"currency"           json:"currency"`
	PayslipLineID *string         `db:"payslip_line_id"    json:"payslip_line_id,omitempty"`
	CreatedAt     time.Time       `db:"created_at"         json:"created_at"`
}

// SubjectRef is the employee identity the checklist engine needs.
type SubjectRef struct {
	EmployeeID    string
	UserID        *string
	ManagerUserID *string
	DisplayName   string
}

// SourceNotice is the notice information a decision record already holds.
// hrm_resignations carries all three; hrm_terminations carries none, which
// is why every field is optional.
type SourceNotice struct {
	NoticePeriodDays  int
	IsNoticeWaived    bool
	LastWorkingDate   time.Time
	HasNoticeTracking bool
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ── Exits ────────────────────────────────────────────────────────────────────

const exitSel = `id, public_id, org_id, employee_id, source_type, source_id,
	last_working_date, expected_last_working_date, notice_shortfall_days,
	checklist_instance_id, fnf_payslip_run_id, status, access_revoked_at,
	remarks, created_by, created_at, updated_at`

func scanExit(row pgx.Row) (*Exit, error) {
	e := &Exit{}
	err := row.Scan(&e.ID, &e.PublicID, &e.OrgID, &e.EmployeeID, &e.SourceType, &e.SourceID,
		&e.LastWorkingDate, &e.ExpectedLastWorkingDate, &e.NoticeShortfallDays,
		&e.ChecklistInstanceID, &e.FnFPayslipRunID, &e.Status, &e.AccessRevokedAt,
		&e.Remarks, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *repoImpl) CreateExit(ctx context.Context, e *Exit) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_exits
		   (org_id, employee_id, source_type, source_id, last_working_date,
		    expected_last_working_date, notice_shortfall_days, remarks, created_by)
		 VALUES ($1,$2::uuid,$3,$4::uuid,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, status, created_at, updated_at`,
		e.OrgID, e.EmployeeID, e.SourceType, e.SourceID, e.LastWorkingDate,
		e.ExpectedLastWorkingDate, e.NoticeShortfallDays, e.Remarks, e.CreatedBy,
	).Scan(&e.ID, &e.PublicID, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("exits: CreateExit: %w", err)
	}
	return nil
}

func (r *repoImpl) FindExitByRef(ctx context.Context, orgID, ref string) (*Exit, error) {
	return scanExit(r.db.QueryRow(ctx,
		`SELECT `+exitSel+` FROM hrm_exits WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindOpenExitForEmployee(ctx context.Context, orgID, employeeID string) (*Exit, error) {
	// Matches uq_hrm_exit_active exactly. Checking in Go as well as relying
	// on the index turns a raw 23505 into ErrExitAlreadyOpen.
	return scanExit(r.db.QueryRow(ctx,
		`SELECT `+exitSel+` FROM hrm_exits
		  WHERE org_id=$1 AND employee_id=$2::uuid AND status NOT IN ('completed','cancelled')`,
		orgID, employeeID))
}

// exitWhere builds the predicate shared by FindExits and CountExits so a list
// and its own total cannot drift.
func exitWhere(orgID string, f ListFilter) (string, []any) {
	args := []any{orgID}
	clauses := []string{"org_id=$1"}
	add := func(clause string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.EmployeeID != "" {
		add("employee_id=$%d::uuid", f.EmployeeID)
	}
	// The scope tier. ScopeNone matches nothing, which is why ListFilter's
	// Scope must always be set explicitly, including by internal callers.
	frag, scopeArgs := scope.Predicate(f.Scope, "employee_id", len(args), orgID, f.CallerUserID, scope.DefaultMaxDepth)
	clauses = append(clauses, "("+frag+")")
	args = append(args, scopeArgs...)
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindExits(ctx context.Context, orgID string, f ListFilter) ([]*Exit, error) {
	f.Normalise()
	where, args := exitWhere(orgID, f)
	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + exitSel + ` FROM hrm_exits` + where +
		fmt.Sprintf(` ORDER BY last_working_date DESC, created_at DESC LIMIT $%d OFFSET $%d`,
			len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("exits: FindExits: %w", err)
	}
	defer rows.Close()
	list := make([]*Exit, 0)
	for rows.Next() {
		e, err := scanExit(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountExits(ctx context.Context, orgID string, f ListFilter) (int, error) {
	where, args := exitWhere(orgID, f)
	var n int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_exits`+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("exits: CountExits: %w", err)
	}
	return n, nil
}

func (r *repoImpl) UpdateExit(ctx context.Context, e *Exit) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_exits
		    SET last_working_date=$3, expected_last_working_date=$4, notice_shortfall_days=$5,
		        checklist_instance_id=$6, fnf_payslip_run_id=$7, status=$8,
		        access_revoked_at=$9, remarks=$10, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		e.OrgID, e.ID, e.LastWorkingDate, e.ExpectedLastWorkingDate, e.NoticeShortfallDays,
		e.ChecklistInstanceID, e.FnFPayslipRunID, e.Status, e.AccessRevokedAt, e.Remarks)
	if err != nil {
		return fmt.Errorf("exits: UpdateExit: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrExitNotFound
	}
	return nil
}

// ── Source validation ────────────────────────────────────────────────────────

func (r *repoImpl) SourceExists(ctx context.Context, orgID string, sourceType SourceType, sourceID, employeeID string) (bool, error) {
	// Two separate statements rather than a UNION: the tables are unrelated
	// and a union would have to agree on a column list they do not share.
	var q string
	switch sourceType {
	case SourceResignation:
		q = `SELECT COUNT(*) FROM hrm_resignations WHERE org_id=$1 AND id=$2::uuid AND employee_id=$3::uuid`
	case SourceTermination:
		q = `SELECT COUNT(*) FROM hrm_terminations WHERE org_id=$1 AND id=$2::uuid AND employee_id=$3::uuid`
	default:
		return false, ErrInvalidSourceType
	}
	var n int
	if err := r.db.QueryRow(ctx, q, orgID, sourceID, employeeID).Scan(&n); err != nil {
		return false, fmt.Errorf("exits: SourceExists: %w", err)
	}
	return n > 0, nil
}

func (r *repoImpl) FindSourceNotice(ctx context.Context, orgID string, sourceType SourceType, sourceID string) (*SourceNotice, error) {
	// Only resignations track notice. A termination's last working date is
	// set by the employer, so there is no entitlement to fall short of —
	// returning "no notice tracking" here is what makes the shortfall zero.
	if sourceType != SourceResignation {
		return &SourceNotice{HasNoticeTracking: false}, nil
	}
	sn := &SourceNotice{HasNoticeTracking: true}
	err := r.db.QueryRow(ctx,
		`SELECT notice_period_days, is_notice_waived, last_working_date
		   FROM hrm_resignations WHERE org_id=$1 AND id=$2::uuid`,
		orgID, sourceID).Scan(&sn.NoticePeriodDays, &sn.IsNoticeWaived, &sn.LastWorkingDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: FindSourceNotice: %w", err)
	}
	return sn, nil
}

func (r *repoImpl) FindTerminationRehireFlag(ctx context.Context, orgID, terminationID string) (*bool, error) {
	var flag bool
	err := r.db.QueryRow(ctx,
		`SELECT is_rehire_eligible FROM hrm_terminations WHERE org_id=$1 AND id=$2::uuid`,
		orgID, terminationID).Scan(&flag)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: FindTerminationRehireFlag: %w", err)
	}
	return &flag, nil
}

// ── Clearance ────────────────────────────────────────────────────────────────

const clearanceSel = `ci.id, ci.public_id, ci.exit_id, ci.checklist_item_id, ci.department,
	ci.description, ci.blocking_amount, ci.currency, ci.is_resolved, ci.resolved_by,
	ci.resolved_at, ci.resolution_note, ci.created_by, ci.created_at, ci.updated_at`

func scanClearanceItem(row pgx.Row) (*ClearanceItem, error) {
	c := &ClearanceItem{}
	err := row.Scan(&c.ID, &c.PublicID, &c.ExitID, &c.ChecklistItemID, &c.Department,
		&c.Description, &c.BlockingAmount, &c.Currency, &c.IsResolved, &c.ResolvedBy,
		&c.ResolvedAt, &c.ResolutionNote, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *repoImpl) CreateClearanceItem(ctx context.Context, orgID string, item *ClearanceItem) error {
	// The SELECT in the VALUES clause is the tenant check: an exit id from
	// another org yields no row rather than attaching a debt across a tenant
	// boundary. Same shape as tickets' CreateComment.
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_exit_clearance_items
		   (exit_id, checklist_item_id, department, description, blocking_amount, currency, created_by)
		 SELECT e.id, $3::uuid, $4, $5, $6, $7, $8 FROM hrm_exits e
		  WHERE e.id=$2::uuid AND e.org_id=$1
		 RETURNING id, public_id, is_resolved, created_at, updated_at`,
		orgID, item.ExitID, item.ChecklistItemID, item.Department, item.Description,
		item.BlockingAmount, item.Currency, item.CreatedBy,
	).Scan(&item.ID, &item.PublicID, &item.IsResolved, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrExitNotFound
	}
	if err != nil {
		return fmt.Errorf("exits: CreateClearanceItem: %w", err)
	}
	return nil
}

func (r *repoImpl) FindClearanceItems(ctx context.Context, orgID, exitID string) ([]*ClearanceItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+clearanceSel+` FROM hrm_exit_clearance_items ci
		   JOIN hrm_exits e ON e.id = ci.exit_id
		  WHERE e.org_id=$1 AND ci.exit_id=$2::uuid
		  ORDER BY ci.is_resolved, ci.department, ci.created_at`,
		orgID, exitID)
	if err != nil {
		return nil, fmt.Errorf("exits: FindClearanceItems: %w", err)
	}
	defer rows.Close()
	list := make([]*ClearanceItem, 0)
	for rows.Next() {
		c, err := scanClearanceItem(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindClearanceItemByRef(ctx context.Context, orgID, exitID, ref string) (*ClearanceItem, error) {
	return scanClearanceItem(r.db.QueryRow(ctx,
		`SELECT `+clearanceSel+` FROM hrm_exit_clearance_items ci
		   JOIN hrm_exits e ON e.id = ci.exit_id
		  WHERE e.org_id=$1 AND ci.exit_id=$2::uuid AND (ci.id::text=$3 OR ci.public_id=$3)`,
		orgID, exitID, ref))
}

func (r *repoImpl) UpdateClearanceItem(ctx context.Context, orgID string, item *ClearanceItem) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_exit_clearance_items ci
		    SET blocking_amount=$3, is_resolved=$4, resolved_by=$5, resolved_at=$6,
		        resolution_note=$7, updated_at=NOW()
		   FROM hrm_exits e
		  WHERE ci.exit_id = e.id AND e.org_id=$1 AND ci.id=$2::uuid`,
		orgID, item.ID, item.BlockingAmount, item.IsResolved, item.ResolvedBy,
		item.ResolvedAt, item.ResolutionNote)
	if err != nil {
		return fmt.Errorf("exits: UpdateClearanceItem: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrClearanceItemNotFound
	}
	return nil
}

// ── Rehire eligibility ───────────────────────────────────────────────────────

const rehireSel = `id, public_id, org_id, employee_id, exit_id, status, reason,
	decided_by, decided_at, created_at, updated_at`

// rehireSelQualified is the same list aliased for the JOIN in
// FindRehireEligibilityByEmail. Written out rather than derived from
// rehireSel by string surgery: "id," appears inside public_id, org_id,
// employee_id and exit_id too, so a replace mangles every one of them.
const rehireSelQualified = `rh.id, rh.public_id, rh.org_id, rh.employee_id, rh.exit_id,
	rh.status, rh.reason, rh.decided_by, rh.decided_at, rh.created_at, rh.updated_at`

func (r *repoImpl) UpsertRehireEligibility(ctx context.Context, re *RehireEligibility) error {
	// ON CONFLICT against uq_hrm_rhe_employee: one standing decision per
	// person. A later exit revises it rather than adding a second row a
	// recruiter would have to adjudicate between.
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_rehire_eligibility (org_id, employee_id, exit_id, status, reason, decided_by)
		 VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6::uuid)
		 ON CONFLICT (employee_id) DO UPDATE
		    SET status=EXCLUDED.status, reason=EXCLUDED.reason,
		        exit_id=COALESCE(EXCLUDED.exit_id, hrm_rehire_eligibility.exit_id),
		        decided_by=EXCLUDED.decided_by, decided_at=NOW(), updated_at=NOW()
		 RETURNING id, public_id, decided_at, created_at, updated_at`,
		re.OrgID, re.EmployeeID, re.ExitID, re.Status, re.Reason, re.DecidedBy,
	).Scan(&re.ID, &re.PublicID, &re.DecidedAt, &re.CreatedAt, &re.UpdatedAt)
	if err != nil {
		return fmt.Errorf("exits: UpsertRehireEligibility: %w", err)
	}
	return nil
}

func (r *repoImpl) FindRehireEligibility(ctx context.Context, orgID, employeeID string) (*RehireEligibility, error) {
	re := &RehireEligibility{}
	err := r.db.QueryRow(ctx,
		`SELECT `+rehireSel+` FROM hrm_rehire_eligibility WHERE org_id=$1 AND employee_id=$2::uuid`,
		orgID, employeeID,
	).Scan(&re.ID, &re.PublicID, &re.OrgID, &re.EmployeeID, &re.ExitID, &re.Status,
		&re.Reason, &re.DecidedBy, &re.DecidedAt, &re.CreatedAt, &re.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: FindRehireEligibility: %w", err)
	}
	return re, nil
}

func (r *repoImpl) FindRehireEligibilityByEmail(ctx context.Context, orgID, email string) (*RehireEligibility, error) {
	// Joins through hrm_employees on either the work or the personal address.
	// Case-insensitive: a candidate typing their address differently from
	// however HR recorded it is the common case, not the exception.
	re := &RehireEligibility{}
	err := r.db.QueryRow(ctx,
		`SELECT `+rehireSelQualified+`
		   FROM hrm_rehire_eligibility rh
		   JOIN hrm_employees e ON e.id = rh.employee_id
		  WHERE rh.org_id=$1
		    AND (LOWER(e.work_email) = LOWER($2) OR LOWER(e.email) = LOWER($2))
		  LIMIT 1`,
		orgID, email,
	).Scan(&re.ID, &re.PublicID, &re.OrgID, &re.EmployeeID, &re.ExitID, &re.Status,
		&re.Reason, &re.DecidedBy, &re.DecidedAt, &re.CreatedAt, &re.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: FindRehireEligibilityByEmail: %w", err)
	}
	return re, nil
}

func (r *repoImpl) FindSubject(ctx context.Context, orgID, employeeRef string) (*SubjectRef, error) {
	ref := &SubjectRef{}
	err := r.db.QueryRow(ctx,
		`SELECT e.id::text, e.user_id::text, m.user_id::text,
		        TRIM(COALESCE(e.first_name,'') || ' ' || COALESCE(e.last_name,''))
		   FROM hrm_employees e
		   LEFT JOIN hrm_employees m ON m.id = e.manager_id
		  WHERE e.org_id=$1 AND (e.id::text=$2 OR e.public_id=$2)`,
		orgID, employeeRef,
	).Scan(&ref.EmployeeID, &ref.UserID, &ref.ManagerUserID, &ref.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: FindSubject: %w", err)
	}
	return ref, nil
}

func (r *repoImpl) FindEmployeeIDByUserID(ctx context.Context, orgID, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM hrm_employees WHERE org_id=$1 AND user_id=$2::uuid LIMIT 1`,
		orgID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("exits: FindEmployeeIDByUserID: %w", err)
	}
	return id, nil
}

// summariseClearance folds items into the derived summary. Kept next to the
// queries that produce the items so the derivation is visible alongside the
// data, and deliberately NOT a SQL aggregate — a stored or cached total is
// exactly what migration 00114 forbids.
func summariseClearance(items []*ClearanceItem) *ClearanceSummary {
	s := &ClearanceSummary{OutstandingDues: decimal.Zero}
	for _, it := range items {
		s.TotalItems++
		if it.IsResolved {
			s.ResolvedItems++
			continue
		}
		// Unresolved. Only a non-zero amount BLOCKS; an outstanding "return
		// your badge" step is incomplete but owes nothing.
		if it.BlockingAmount.IsPositive() {
			s.BlockingItems++
			s.OutstandingDues = s.OutstandingDues.Add(it.BlockingAmount)
		}
	}
	s.IsComplete = s.TotalItems > 0 && s.ResolvedItems == s.TotalItems
	return s
}

// ── Settlement (9B) ──────────────────────────────────────────────────────────

func (r *repoImpl) FindExitByFnFRun(ctx context.Context, orgID, runID string) (*Exit, error) {
	return scanExit(r.db.QueryRow(ctx,
		`SELECT `+exitSel+` FROM hrm_exits WHERE org_id=$1 AND fnf_payslip_run_id=$2::uuid`,
		orgID, runID))
}

func (r *repoImpl) FindExitByFnFRunAnyOrg(ctx context.Context, runID string) (*Exit, error) {
	return scanExit(r.db.QueryRow(ctx,
		`SELECT `+exitSel+` FROM hrm_exits WHERE fnf_payslip_run_id=$1::uuid`, runID))
}

func (r *repoImpl) FnFRunCurrency(ctx context.Context, orgID, runID string) (string, error) {
	var currency string
	err := r.db.QueryRow(ctx,
		`SELECT currency FROM hrm_payslip_runs WHERE org_id=$1 AND id=$2::uuid`,
		orgID, runID).Scan(&currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("exits: FnFRunCurrency: %w", err)
	}
	return currency, nil
}

const gratuitySel = `id, public_id, org_id, name, min_years_of_service, days_per_year,
	base_component, monthly_divisor, forfeit_on_misconduct, effective_date,
	created_by, created_at, updated_at`

func scanGratuityRule(row pgx.Row) (*GratuityRuleRow, error) {
	g := &GratuityRuleRow{}
	err := row.Scan(&g.ID, &g.PublicID, &g.OrgID, &g.Name, &g.MinYearsOfService, &g.DaysPerYear,
		&g.BaseComponent, &g.MonthlyDivisor, &g.ForfeitOnMisconduct, &g.EffectiveDate,
		&g.CreatedBy, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (r *repoImpl) GratuityRuleAsOf(ctx context.Context, orgID string, asOf time.Time) (*GratuityRuleRow, error) {
	// The effective-dated lookup, the 7D SlabsAsOf shape. The subquery is
	// what makes "the rule in force" well-defined: without the
	// `effective_date <= asOf` bound a rule dated next month would win, and
	// a settlement computed today would silently use terms that do not yet
	// apply.
	return scanGratuityRule(r.db.QueryRow(ctx,
		`SELECT `+gratuitySel+` FROM hrm_gratuity_rules
		  WHERE org_id=$1 AND effective_date = (
		      SELECT MAX(effective_date) FROM hrm_gratuity_rules
		       WHERE org_id=$1 AND effective_date <= $2
		  )
		  ORDER BY created_at DESC
		  LIMIT 1`,
		orgID, asOf))
}

func (r *repoImpl) CreateGratuityRule(ctx context.Context, g *GratuityRuleRow) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_gratuity_rules
		   (org_id, name, min_years_of_service, days_per_year, base_component,
		    monthly_divisor, forfeit_on_misconduct, effective_date, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, public_id, created_at, updated_at`,
		g.OrgID, g.Name, g.MinYearsOfService, g.DaysPerYear, g.BaseComponent,
		g.MonthlyDivisor, g.ForfeitOnMisconduct, g.EffectiveDate, g.CreatedBy,
	).Scan(&g.ID, &g.PublicID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return fmt.Errorf("exits: CreateGratuityRule: %w", err)
	}
	return nil
}

func (r *repoImpl) ListGratuityRules(ctx context.Context, orgID string) ([]*GratuityRuleRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+gratuitySel+` FROM hrm_gratuity_rules WHERE org_id=$1
		  ORDER BY effective_date DESC, created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("exits: ListGratuityRules: %w", err)
	}
	defer rows.Close()
	list := make([]*GratuityRuleRow, 0)
	for rows.Next() {
		g, err := scanGratuityRule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

func (r *repoImpl) EmployeeTenure(ctx context.Context, orgID, employeeID string) (*EmployeeTenure, error) {
	// Monthly pay comes from the latest salary record, the same LATERAL the
	// payroll engine uses — so gratuity and the payslip agree on what the
	// person was earning.
	t := &EmployeeTenure{}
	var hire *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT e.hire_date, COALESCE(es.basic_pay, 0),
		        EXISTS (SELECT 1 FROM hrm_terminations t
		                 WHERE t.employee_id = e.id AND t.org_id = e.org_id
		                   AND t.termination_type = 'involuntary')
		   FROM hrm_employees e
		   LEFT JOIN LATERAL (
		       SELECT basic_pay FROM hrm_employee_salary_records
		        WHERE employee_id = e.id ORDER BY effective_date DESC LIMIT 1
		   ) es ON TRUE
		  WHERE e.org_id=$1 AND e.id=$2::uuid`,
		orgID, employeeID).Scan(&hire, &t.MonthlyPay, &t.IsForCause)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exits: EmployeeTenure: %w", err)
	}
	if hire != nil {
		t.HireDate = *hire
	}
	return t, nil
}

func (r *repoImpl) InsertSettlementLines(ctx context.Context, exitID string, lines []SettlementLineRow) error {
	if len(lines) == 0 {
		return nil
	}
	// Replace-then-insert inside one transaction: re-assembling a settlement
	// must SUPERSEDE the previous attempt, not append alongside it.
	//
	// The DELETE is unconditional, and an earlier version that spared rows
	// with a payslip_line_id was wrong: a spared row is not refreshed by the
	// insert that follows, so the same claim ended up on the trail twice and
	// every figure read double. MarkSettled re-stamps the links immediately
	// after the payslip lines are written, so nothing is lost by clearing
	// them here — and payslip_line_id is ON DELETE SET NULL anyway, so a
	// link whose payslip line has been removed is already NULL.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("exits: InsertSettlementLines: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM hrm_exit_settlement_lines WHERE exit_id=$1::uuid`,
		exitID); err != nil {
		return fmt.Errorf("exits: InsertSettlementLines: clear previous: %w", err)
	}
	for _, l := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO hrm_exit_settlement_lines
			   (exit_id, source_type, source_id, description, amount, is_credit, currency)
			 VALUES ($1::uuid,$2,$3,$4,$5,$6,$7)`,
			exitID, l.SourceType, l.SourceID, l.Description, l.Amount, l.IsCredit, l.Currency); err != nil {
			return fmt.Errorf("exits: InsertSettlementLines: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) FindSettlementLines(ctx context.Context, orgID, exitID string) ([]*SettlementLineRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT sl.id, sl.public_id, sl.exit_id, sl.source_type, sl.source_id, sl.description,
		        sl.amount, sl.is_credit, sl.currency, sl.payslip_line_id, sl.created_at
		   FROM hrm_exit_settlement_lines sl
		   JOIN hrm_exits e ON e.id = sl.exit_id
		  WHERE e.org_id=$1 AND sl.exit_id=$2::uuid
		  ORDER BY sl.is_credit DESC, sl.created_at`,
		orgID, exitID)
	if err != nil {
		return nil, fmt.Errorf("exits: FindSettlementLines: %w", err)
	}
	defer rows.Close()
	list := make([]*SettlementLineRow, 0)
	for rows.Next() {
		l := &SettlementLineRow{}
		if err := rows.Scan(&l.ID, &l.PublicID, &l.ExitID, &l.SourceType, &l.SourceID,
			&l.Description, &l.Amount, &l.IsCredit, &l.Currency, &l.PayslipLineID, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

func (r *repoImpl) LinkSettlementLineToPayslip(ctx context.Context, exitID, sourceType, sourceID, payslipLineID string) error {
	// Matched on (exit, source_type, source_id) rather than an id the caller
	// would have to carry through the payroll engine — payslips knows the
	// source that produced a line, not this table's primary key.
	var q string
	var args []any
	if sourceID == "" {
		q = `UPDATE hrm_exit_settlement_lines SET payslip_line_id=$3::uuid
		      WHERE exit_id=$1::uuid AND source_type=$2 AND source_id IS NULL AND payslip_line_id IS NULL`
		args = []any{exitID, sourceType, payslipLineID}
	} else {
		q = `UPDATE hrm_exit_settlement_lines SET payslip_line_id=$4::uuid
		      WHERE exit_id=$1::uuid AND source_type=$2 AND source_id=$3::uuid AND payslip_line_id IS NULL`
		args = []any{exitID, sourceType, sourceID, payslipLineID}
	}
	if _, err := r.db.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("exits: LinkSettlementLineToPayslip: %w", err)
	}
	return nil
}

// ── Exit interviews (9C) ─────────────────────────────────────────────────────

const interviewSel = `id, public_id, org_id, exit_id, form_instance_id, status,
	scheduled_for, sent_at, completed_at, created_by, created_at, updated_at`

func scanInterview(row pgx.Row) (*ExitInterview, error) {
	i := &ExitInterview{}
	err := row.Scan(&i.ID, &i.PublicID, &i.OrgID, &i.ExitID, &i.FormInstanceID, &i.Status,
		&i.ScheduledFor, &i.SentAt, &i.CompletedAt, &i.CreatedBy, &i.CreatedAt, &i.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *repoImpl) CreateInterview(ctx context.Context, i *ExitInterview) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO hrm_exit_interviews (org_id, exit_id, scheduled_for, created_by)
		 VALUES ($1,$2::uuid,$3,$4)
		 RETURNING id, public_id, status, created_at, updated_at`,
		i.OrgID, i.ExitID, i.ScheduledFor, i.CreatedBy,
	).Scan(&i.ID, &i.PublicID, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return fmt.Errorf("exits: CreateInterview: %w", err)
	}
	return nil
}

func (r *repoImpl) FindInterviewByExit(ctx context.Context, orgID, exitID string) (*ExitInterview, error) {
	return scanInterview(r.db.QueryRow(ctx,
		`SELECT `+interviewSel+` FROM hrm_exit_interviews WHERE org_id=$1 AND exit_id=$2::uuid`,
		orgID, exitID))
}

func (r *repoImpl) FindInterviewByRef(ctx context.Context, orgID, ref string) (*ExitInterview, error) {
	return scanInterview(r.db.QueryRow(ctx,
		`SELECT `+interviewSel+` FROM hrm_exit_interviews
		  WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`, orgID, ref))
}

func (r *repoImpl) UpdateInterview(ctx context.Context, i *ExitInterview) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_exit_interviews
		    SET form_instance_id=$3, status=$4, scheduled_for=$5, sent_at=$6,
		        completed_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		i.OrgID, i.ID, i.FormInstanceID, i.Status, i.ScheduledFor, i.SentAt, i.CompletedAt)
	if err != nil {
		return fmt.Errorf("exits: UpdateInterview: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrInterviewNotFound
	}
	return nil
}

func (r *repoImpl) FindDueInterviews(ctx context.Context, asOf time.Time, limit int) ([]*ExitInterview, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+interviewSel+` FROM hrm_exit_interviews
		  WHERE status='scheduled' AND scheduled_for <= $1
		  ORDER BY scheduled_for
		  LIMIT $2`,
		asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("exits: FindDueInterviews: %w", err)
	}
	defer rows.Close()
	list := make([]*ExitInterview, 0)
	for rows.Next() {
		i, err := scanInterview(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, rows.Err()
}

// ── Access revocation (9C) ───────────────────────────────────────────────────

func (r *repoImpl) FindExitsDueForRevocation(ctx context.Context, asOf time.Time, limit int) ([]*Exit, error) {
	// Matches idx_hrm_exit_revocation_due exactly. access_revoked_at IS NULL
	// is what makes the sweep idempotent: a revoked exit drops out of the set
	// permanently rather than being re-revoked every night.
	rows, err := r.db.Query(ctx,
		`SELECT `+exitSel+` FROM hrm_exits
		  WHERE access_revoked_at IS NULL
		    AND status NOT IN ('cancelled')
		    AND last_working_date <= $1
		  ORDER BY last_working_date
		  LIMIT $2`,
		asOf, limit)
	if err != nil {
		return nil, fmt.Errorf("exits: FindExitsDueForRevocation: %w", err)
	}
	defer rows.Close()
	list := make([]*Exit, 0)
	for rows.Next() {
		e, err := scanExit(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *repoImpl) FindEmployeeUserID(ctx context.Context, orgID, employeeID string) (string, error) {
	var userID *string
	err := r.db.QueryRow(ctx,
		`SELECT user_id::text FROM hrm_employees WHERE org_id=$1 AND id=$2::uuid`,
		orgID, employeeID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("exits: FindEmployeeUserID: %w", err)
	}
	if userID == nil {
		return "", nil
	}
	return *userID, nil
}
