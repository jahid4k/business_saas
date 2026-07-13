// src/types/hrm.ts
// All fields mirror backend JSON tags exactly (snake_case) — same convention as types/crm.ts

// ── Departments ───────────────────────────────────────────
export interface Department {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  parent_department_id?: string;
  head_employee_id?: string;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface DepartmentListResponse {
  departments: Department[];
  total: number;
}

export interface CreateDepartmentPayload {
  name: string;
  description?: string;
  parent_department_id?: string;
  head_employee_id?: string;
}

export interface UpdateDepartmentPayload {
  name?: string;
  description?: string;
  parent_department_id?: string;
  head_employee_id?: string;
  is_active?: boolean;
}

// ── Positions ─────────────────────────────────────────────
export interface Position {
  id: string;
  public_id: string;
  org_id: string;
  department_id?: string;
  title: string;
  description?: string;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PositionListResponse {
  positions: Position[];
  total: number;
}

export interface CreatePositionPayload {
  title: string;
  description?: string;
  department_id?: string;
}

export interface UpdatePositionPayload {
  title?: string;
  description?: string;
  department_id?: string;
  is_active?: boolean;
}

// ── Employees ─────────────────────────────────────────────
export type EmploymentType =
  | "full_time"
  | "part_time"
  | "contractor"
  | "intern";
export type EmployeeStatus = "active" | "inactive" | "on_leave" | "terminated";
export type Gender = "male" | "female" | "other" | "prefer_not_to_say";

export interface Employee {
  id: string;
  public_id: string;
  org_id: string;
  user_id?: string;
  employee_number?: string;
  first_name: string;
  last_name?: string;
  email?: string;
  work_email?: string;
  phone?: string;
  work_phone?: string;
  date_of_birth?: string;
  gender?: Gender;
  avatar_url?: string;
  hire_date: string;
  termination_date?: string;
  employment_type: EmploymentType;
  status: EmployeeStatus;
  department_id?: string;
  position_id?: string;
  manager_id?: string;
  address?: string;
  city?: string;
  country?: string;
  notes?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface EmployeeListResponse {
  employees: Employee[];
  total: number;
  limit: number;
  offset: number;
}

export interface EmployeeListFilter {
  status?: EmployeeStatus;
  employment_type?: EmploymentType;
  department_id?: string;
  manager_id?: string;
  search?: string;
  limit?: number;
  offset?: number;
}

export interface CreateEmployeePayload {
  first_name: string;
  last_name?: string;
  email?: string;
  work_email?: string;
  phone?: string;
  work_phone?: string;
  employee_number?: string;
  date_of_birth?: string;
  gender?: Gender;
  hire_date: string;
  employment_type?: EmploymentType;
  department_id?: string;
  position_id?: string;
  manager_id?: string;
  address?: string;
  city?: string;
  country?: string;
  notes?: string;
}

export interface UpdateEmployeePayload {
  first_name?: string;
  last_name?: string;
  email?: string;
  work_email?: string;
  phone?: string;
  work_phone?: string;
  employee_number?: string;
  date_of_birth?: string;
  gender?: Gender;
  employment_type?: EmploymentType;
  status?: EmployeeStatus;
  department_id?: string;
  position_id?: string;
  manager_id?: string;
  address?: string;
  city?: string;
  country?: string;
  notes?: string;
}

export interface TerminateEmployeePayload {
  termination_date: string;
  notes?: string;
}

// ── Leave ─────────────────────────────────────────────────
export type LeaveRequestStatus =
  | "pending"
  | "approved"
  | "rejected"
  | "cancelled";

export interface LeaveType {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  max_days_per_year: number; // 0 = unlimited
  is_paid: boolean;
  requires_approval: boolean;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface LeaveTypeListResponse {
  leave_types: LeaveType[];
  total: number;
}

export interface CreateLeaveTypePayload {
  name: string;
  description?: string;
  max_days_per_year: number;
  is_paid?: boolean;
  requires_approval?: boolean;
}

export interface UpdateLeaveTypePayload {
  name?: string;
  description?: string;
  max_days_per_year?: number;
  is_paid?: boolean;
  requires_approval?: boolean;
  is_active?: boolean;
}

export interface LeaveRequest {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  leave_type_id: string;
  start_date: string;
  end_date: string;
  total_days: number;
  reason?: string;
  status: LeaveRequestStatus;
  reviewed_by?: string;
  reviewed_at?: string;
  review_note?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface LeaveRequestListResponse {
  requests: LeaveRequest[];
  total: number;
  limit: number;
  offset: number;
}

export interface LeaveRequestListFilter {
  employee_id?: string;
  leave_type_id?: string;
  status?: LeaveRequestStatus;
  limit?: number;
  offset?: number;
}

export interface CreateLeaveRequestPayload {
  employee_id: string;
  leave_type_id: string;
  start_date: string;
  end_date: string;
  total_days: number;
  reason?: string;
}

export interface ReviewLeaveRequestPayload {
  note?: string;
}

// ── Employee Lifecycle: Promotions ───────────────────────
export type PromotionStatus =
  | "draft"
  | "pending_approval"
  | "approved"
  | "rejected"
  | "cancelled"
  | "applied";

export interface Promotion {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  from_position_id?: string;
  from_department_id?: string;
  from_basic_pay?: number;
  to_position_id: string;
  to_department_id?: string;
  new_basic_pay?: number;
  effective_date: string;
  reason?: string;
  notes?: string;
  approval_instance_id?: string;
  status: PromotionStatus;
  applied_at?: string;
  applied_by?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PromotionListResponse {
  promotions: Promotion[];
  total: number;
}

export interface CreatePromotionPayload {
  to_position_id: string;
  to_department_id?: string;
  new_basic_pay?: number;
  effective_date: string;
  reason?: string;
  notes?: string;
}

// ── Employee Lifecycle: Transfers ────────────────────────
export type TransferType = "department" | "location" | "reporting" | "full";
export type TransferStatus =
  | "draft"
  | "pending_approval"
  | "approved"
  | "rejected"
  | "cancelled"
  | "applied";

export interface Transfer {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  transfer_type: TransferType;
  from_department_id?: string;
  from_manager_employee_id?: string;
  from_location?: string;
  to_department_id?: string;
  to_manager_employee_id?: string;
  to_location?: string;
  effective_date: string;
  reason?: string;
  notes?: string;
  approval_instance_id?: string;
  status: TransferStatus;
  applied_at?: string;
  applied_by?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface TransferListResponse {
  transfers: Transfer[];
  total: number;
}

export interface CreateTransferPayload {
  transfer_type: TransferType;
  to_department_id?: string;
  to_manager_employee_id?: string;
  to_location?: string;
  effective_date: string;
  reason?: string;
  notes?: string;
}

// ── Employee Lifecycle: Resignations ─────────────────────
export type ResignationStatus =
  | "submitted"
  | "accepted"
  | "withdrawn"
  | "rejected";
export type ResignationReasonCategory =
  | "personal"
  | "career_growth"
  | "better_opportunity"
  | "relocation"
  | "health"
  | "retirement"
  | "other";

export interface Resignation {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  resignation_date: string;
  notice_period_days: number;
  is_notice_waived: boolean;
  last_working_date: string;
  reason_category: ResignationReasonCategory;
  reason_remarks?: string;
  exit_interview_completed: boolean;
  exit_clearance_completed: boolean;
  status: ResignationStatus;
  accepted_at?: string;
  accepted_by?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ResignationListResponse {
  resignations: Resignation[];
  total: number;
}

export interface SubmitResignationPayload {
  resignation_date: string;
  reason_category: ResignationReasonCategory;
  reason_remarks?: string;
  last_working_date?: string;
  is_notice_waived?: boolean;
}

// ── Employee Lifecycle: Terminations ─────────────────────
export type TerminationType =
  | "voluntary"
  | "involuntary"
  | "layoff"
  | "retirement"
  | "contract_end"
  | "probation_fail";
export type TerminationStatus =
  | "draft"
  | "pending_approval"
  | "approved"
  | "rejected"
  | "cancelled"
  | "applied";

export interface Termination {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  termination_type: TerminationType;
  termination_date: string;
  last_working_date: string;
  reason?: string;
  internal_notes?: string;
  severance_amount?: number;
  severance_currency: string;
  is_rehire_eligible: boolean;
  exit_clearance_completed: boolean;
  approval_instance_id?: string;
  status: TerminationStatus;
  applied_at?: string;
  applied_by?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface TerminationListResponse {
  terminations: Termination[];
  total: number;
}

export interface CreateTerminationPayload {
  termination_type: TerminationType;
  termination_date: string;
  last_working_date: string;
  reason?: string;
  internal_notes?: string;
  severance_amount?: number;
  severance_currency?: string;
  is_rehire_eligible?: boolean;
}

// ── Attendance ────────────────────────────────────────────
export type AttendanceDayType =
  | "present"
  | "absent"
  | "half_day"
  | "late"
  | "on_leave"
  | "holiday"
  | "weekend"
  | "work_from_home";
export type AttendanceSource = "manual" | "device" | "api" | "system";
export type AttendanceRecordStatus = "approved" | "pending" | "rejected";
export type AttendancePeriodStatus = "open" | "finalized" | "locked";

export interface AttendanceRecord {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  attendance_date: string;
  shift_id?: string;
  shift_name?: string;
  expected_in?: string;
  expected_out?: string;
  check_in_time?: string;
  check_out_time?: string;
  break_minutes: number;
  regular_hours: number;
  overtime_hours: number;
  day_type: AttendanceDayType;
  source: AttendanceSource;
  notes?: string;
  regularization_reason?: string;
  status: AttendanceRecordStatus;
  approved_by?: string;
  approved_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface AttendanceRecordListResponse {
  records: AttendanceRecord[];
  total: number;
}

export interface CreateAttendanceRecordPayload {
  employee_id: string;
  date: string;
  check_in?: string;
  check_out?: string;
  break_minutes?: number;
  day_type: AttendanceDayType;
  source?: AttendanceSource;
  notes?: string;
}

export interface RegularizeAttendancePayload {
  new_check_in?: string;
  new_check_out?: string;
  reason: string;
}

export interface AttendancePeriod {
  id: string;
  public_id: string;
  org_id: string;
  period_year: number;
  period_month: number;
  status: AttendancePeriodStatus;
  total_employees: number;
  total_work_days: number;
  total_present: number;
  total_absent: number;
  total_holidays: number;
  total_leaves: number;
  total_overtime_hours: number;
  finalized_at?: string;
  locked_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface AttendancePeriodListResponse {
  periods: AttendancePeriod[];
  total: number;
}

// ── Compliance: Complaints ────────────────────────────────
export type ComplaintType =
  | "harassment"
  | "discrimination"
  | "workplace_safety"
  | "policy_violation"
  | "manager_conduct"
  | "wage_dispute"
  | "retaliation"
  | "general";
export type ComplaintStatus =
  | "submitted"
  | "under_review"
  | "investigating"
  | "resolved"
  | "dismissed"
  | "withdrawn";

export interface Complaint {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  is_anonymous: boolean;
  complaint_type: ComplaintType;
  title: string;
  description: string;
  incident_date?: string;
  against_employee_id?: string;
  against_details?: string;
  investigator_id?: string;
  investigation_notes?: string;
  resolution?: string;
  resolution_action?: string;
  resolved_at?: string;
  resolved_by?: string;
  status: ComplaintStatus;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ComplaintListResponse {
  complaints: Complaint[];
  total: number;
}

export interface CreateComplaintPayload {
  complaint_type: ComplaintType;
  title: string;
  description: string;
  incident_date?: string;
  against_employee_id?: string;
  against_details?: string;
  is_anonymous?: boolean;
}

export interface AssignComplaintPayload {
  investigator_id: string;
}

export interface ResolveComplaintPayload {
  resolution: string;
  resolution_action?: string;
}

export interface DismissComplaintPayload {
  resolution: string;
}

// ── Compliance: Employee Documents ───────────────────────
export type EmployeeDocumentStatus =
  | "draft"
  | "sent"
  | "acknowledged"
  | "declined"
  | "expired"
  | "withdrawn"
  | "superseded";

export interface EmployeeDocument {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  template_id?: string;
  title: string;
  document_type: string;
  file_url: string;
  file_name: string;
  mime_type: string;
  expiry_date?: string;
  status: EmployeeDocumentStatus;
  issued_by?: string;
  sent_at?: string;
  acknowledged_at?: string;
  acknowledgement_note?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface EmployeeDocumentListResponse {
  documents: EmployeeDocument[];
  total: number;
}

export interface CreateEmployeeDocumentPayload {
  title: string;
  document_type: string;
  file_url: string;
  file_name: string;
  mime_type: string;
  expiry_date?: string;
}

// ── Compliance: Acknowledgements ─────────────────────────
export type AcknowledgementType =
  | "warning"
  | "document"
  | "announcement"
  | "calendar_event"
  | "policy";
export type AcknowledgementStatus =
  | "pending"
  | "acknowledged"
  | "declined"
  | "expired";

export interface Acknowledgement {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  acknowledgeable_type: AcknowledgementType;
  acknowledgeable_id: string;
  entity_title: string;
  notes?: string;
  signature_required: boolean;
  signed_at?: string;
  status: AcknowledgementStatus;
  acknowledged_at?: string;
  declined_at?: string;
  decline_reason?: string;
  expires_at?: string;
  requested_by: string;
  requested_at: string;
  created_at: string;
  updated_at: string;
}

export interface AcknowledgementListResponse {
  acknowledgements: Acknowledgement[];
  total: number;
}

export interface CreateAcknowledgementPayload {
  employee_id: string;
  acknowledgeable_type: AcknowledgementType;
  acknowledgeable_id: string;
  entity_title: string;
  signature_required?: boolean;
  expires_at?: string;
}

export interface RespondAcknowledgementPayload {
  notes?: string;
  signature_data?: string;
}

export interface DeclineAcknowledgementPayload {
  reason: string;
}

// ── Shared scope type (Announcements + Calendar) ──────────
export type HrScopeType = "organization" | "department" | "individual";

// ── Recognition: Awards ───────────────────────────────────
export type AwardType =
  | "spot_recognition"
  | "performance"
  | "tenure"
  | "team"
  | "innovation"
  | "customer_service"
  | "custom";
export type AwardStatus =
  | "draft"
  | "pending_approval"
  | "approved"
  | "issued"
  | "cancelled";

export interface Award {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  award_type: AwardType;
  title: string;
  description: string;
  points: number;
  monetary_value?: number;
  currency: string;
  award_date: string;
  issued_by: string;
  certificate_document_id?: string;
  announcement_id?: string;
  approval_instance_id?: string;
  status: AwardStatus;
  issued_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface AwardListResponse {
  awards: Award[];
  total: number;
}

export interface CreateAwardPayload {
  employee_id: string;
  award_type: AwardType;
  title: string;
  description: string;
  points?: number;
  monetary_value?: number;
  currency?: string;
  award_date?: string;
}

export interface IssueAwardPayload {
  create_announcement?: boolean;
  announcement_content?: string;
}

// ── Recognition: Announcements ────────────────────────────
export type AnnouncementCategory =
  | "general"
  | "policy"
  | "event"
  | "award"
  | "reminder"
  | "emergency"
  | "hr_update";
export type AnnouncementStatus =
  | "draft"
  | "scheduled"
  | "published"
  | "expired"
  | "archived";

export interface Announcement {
  id: string;
  public_id: string;
  org_id: string;
  title: string;
  content: string;
  category: AnnouncementCategory;
  scope_type: HrScopeType;
  scope_ids: string[];
  scheduled_at?: string;
  published_at?: string;
  expires_at?: string;
  requires_acknowledgement: boolean;
  acknowledgement_deadline?: string;
  is_pinned: boolean;
  author_id: string;
  status: AnnouncementStatus;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface AnnouncementListResponse {
  announcements: Announcement[];
  total: number;
}

export interface CreateAnnouncementPayload {
  title: string;
  content: string;
  category: AnnouncementCategory;
  scope_type: HrScopeType;
  scope_ids?: string[];
  scheduled_at?: string;
  expires_at?: string;
  requires_acknowledgement?: boolean;
  acknowledgement_deadline?: string;
  is_pinned?: boolean;
}

// ── Recognition: Calendar ─────────────────────────────────
export type CalendarEventType =
  | "holiday"
  | "training"
  | "company_event"
  | "team_event"
  | "deadline"
  | "birthday"
  | "work_anniversary"
  | "custom";
export type CalendarEventStatus =
  | "upcoming"
  | "ongoing"
  | "completed"
  | "cancelled";

export interface CalendarEvent {
  id: string;
  public_id: string;
  org_id: string;
  title: string;
  description?: string;
  event_type: CalendarEventType;
  start_date: string;
  end_date: string;
  is_all_day: boolean;
  start_time?: string;
  end_time?: string;
  location?: string;
  scope_type: HrScopeType;
  scope_ids: string[];
  requires_rsvp: boolean;
  rsvp_deadline?: string;
  organizer_id?: string;
  is_auto_generated: boolean;
  status: CalendarEventStatus;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CalendarEventListResponse {
  events: CalendarEvent[];
  total: number;
}

export interface CreateCalendarEventPayload {
  title: string;
  description?: string;
  event_type: CalendarEventType;
  start_date: string;
  end_date: string;
  is_all_day?: boolean;
  start_time?: string;
  end_time?: string;
  location?: string;
  scope_type: HrScopeType;
  scope_ids?: string[];
  requires_rsvp?: boolean;
  rsvp_deadline?: string;
}

// ── Recognition: Milestones ───────────────────────────────
export type MilestoneType =
  | "work_anniversary"
  | "birthday"
  | "probation_complete"
  | "promotion"
  | "contract_renewal"
  | "retirement"
  | "custom";

export interface Milestone {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  milestone_type: MilestoneType;
  title: string;
  description?: string;
  milestone_date: string;
  years_count?: number;
  is_auto_generated: boolean;
  is_acknowledged: boolean;
  acknowledged_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface MilestoneListResponse {
  milestones: Milestone[];
  total: number;
}

export interface CreateMilestonePayload {
  employee_id: string;
  milestone_type: MilestoneType;
  title: string;
  description?: string;
  milestone_date: string;
  years_count?: number;
  create_award?: boolean;
  create_announcement?: boolean;
  create_calendar_event?: boolean;
}

export interface GenerateMilestonesPayload {
  year: number;
  month: number;
  include_anniversaries?: boolean;
  include_birthdays?: boolean;
  include_probation?: boolean;
  include_contract_renewals?: boolean;
}

export interface GenerateMilestonesResult {
  generated: number;
  skipped: number;
  errors?: string[];
}

// ── Setup: Salary ─────────────────────────────────────────
export type SalaryComponentType =
  | "earning"
  | "deduction"
  | "employer_contribution";
export type SalaryCalcMethod =
  | "fixed"
  | "pct_of_basic"
  | "pct_of_gross"
  | "formula"
  | "manual"
  | "slab";
export type SalaryChangeReason =
  | "joining"
  | "promotion"
  | "annual_revision"
  | "transfer"
  | "correction"
  | "other";

export interface SalarySlab {
  up_to?: number;
  rate: number;
}

export interface SalarySlabConfig {
  base_variable: string;
  slabs: SalarySlab[];
}

export interface SalaryComponent {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  component_type: SalaryComponentType;
  calc_method: SalaryCalcMethod;
  fixed_value: number;
  formula_expression?: string;
  formula_variables?: string[];
  slab_config?: SalarySlabConfig;
  is_taxable: boolean;
  display_order: number;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SalaryComponentListResponse {
  components: SalaryComponent[];
  total: number;
}

export interface CreateSalaryComponentPayload {
  name: string;
  description?: string;
  component_type: SalaryComponentType;
  calc_method: SalaryCalcMethod;
  fixed_value?: number;
  formula_expression?: string;
  slab_config?: SalarySlabConfig;
  is_taxable?: boolean;
  display_order?: number;
}

export interface UpdateSalaryComponentPayload extends Partial<CreateSalaryComponentPayload> {
  is_active?: boolean;
}

export interface StructureComponent {
  component_id: string;
  component?: SalaryComponent;
  override_value?: number;
  display_order: number;
}

export interface SalaryStructure {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  grade_label?: string;
  is_active: boolean;
  components?: StructureComponent[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface SalaryStructureListResponse {
  structures: SalaryStructure[];
  total: number;
}

export interface CreateSalaryStructurePayload {
  name: string;
  description?: string;
  grade_label?: string;
}

export interface AddComponentToStructurePayload {
  component_id: string;
  override_value?: number;
  display_order?: number;
}

export interface EmployeeSalaryRecord {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  structure_id?: string;
  basic_pay: number;
  effective_date: string;
  change_reason: SalaryChangeReason;
  change_notes?: string;
  structure?: SalaryStructure;
  created_by: string;
  created_at: string;
}

export interface SalaryHistoryResponse {
  records: EmployeeSalaryRecord[];
  total: number;
}

export interface AssignSalaryPayload {
  structure_id?: string;
  basic_pay: number;
  effective_date: string;
  change_reason: SalaryChangeReason;
  change_notes?: string;
}

export interface TestFormulaPayload {
  expression: string;
  variables: Record<string, number>;
}

export interface TestFormulaResult {
  result: number;
  valid: boolean;
  error?: string;
}

// ── Payroll ────────────────────────────────────────────────
export type PayslipRunStatus =
  | "draft"
  | "computing"
  | "computed"
  | "approved"
  | "paid"
  | "cancelled";
export type PayslipStatus = "draft" | "computed" | "approved" | "paid";

export interface PayslipRun {
  id: string;
  public_id: string;
  org_id: string;
  period_year: number;
  period_month: number;
  description?: string;
  currency: string;
  attendance_period_id?: string;
  total_employees: number;
  total_gross_pay: number;
  total_deductions: number;
  total_net_pay: number;
  status: PayslipRunStatus;
  computed_at?: string;
  approved_at?: string;
  paid_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PayslipRunListResponse {
  runs: PayslipRun[];
  total: number;
}

export interface CreatePayslipRunPayload {
  year: number;
  month: number;
  description?: string;
  currency?: string;
  attendance_period_id?: string;
}

export interface PayslipLine {
  id: string;
  payslip_id: string;
  org_id: string;
  component_id?: string;
  component_name: string;
  component_type: string;
  calc_method: string;
  formula_used?: string;
  computed_amount: number;
  display_order: number;
}

export interface Payslip {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  payslip_run_id: string;
  period_year: number;
  period_month: number;
  salary_structure_id?: string;
  salary_structure_name?: string;
  gross_pay: number;
  total_deductions: number;
  net_pay: number;
  basic_pay: number;
  work_days: number;
  present_days: number;
  absent_days: number;
  leave_days: number;
  holiday_days: number;
  overtime_hours: number;
  currency: string;
  status: PayslipStatus;
  payment_reference?: string;
  payment_date?: string;
  lines?: PayslipLine[];
  created_at: string;
  updated_at: string;
}

export interface PayslipListResponse {
  payslips: Payslip[];
  total: number;
}

// ── Setup: Warning Types ──────────────────────────────────
export type EscalationAction =
  | "notify_hr"
  | "notify_management"
  | "flag_termination_review";

export interface WarningType {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  severity_level: number;
  can_be_issued_by: string[];
  requires_hr_approval: boolean;
  employee_can_respond: boolean;
  response_window_days: number;
  auto_generate_document: boolean;
  valid_duration_days: number;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface WarningTypeListResponse {
  warning_types: WarningType[];
  total: number;
}

export interface CreateWarningTypePayload {
  name: string;
  description?: string;
  severity_level?: number;
  can_be_issued_by?: string[];
  requires_hr_approval?: boolean;
  employee_can_respond?: boolean;
  response_window_days?: number;
  auto_generate_document?: boolean;
  valid_duration_days?: number;
}

export interface UpdateWarningTypePayload extends Partial<CreateWarningTypePayload> {
  is_active?: boolean;
}

export interface WarningEscalationRule {
  id: string;
  public_id: string;
  org_id: string;
  trigger_warning_type_id: string;
  trigger_count: number;
  within_days: number;
  action: EscalationAction;
  notification_roles: string[];
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface EscalationRuleListResponse {
  rules: WarningEscalationRule[];
  total: number;
}

export interface CreateEscalationRulePayload {
  trigger_warning_type_id: string;
  trigger_count: number;
  within_days?: number;
  action: EscalationAction;
  notification_roles?: string[];
}

// ── Warnings ───────────────────────────────────────────────
export type WarningStatus =
  | "draft"
  | "pending_approval"
  | "issued"
  | "acknowledged"
  | "appealed"
  | "closed"
  | "cancelled";

export interface EmployeeWarning {
  id: string;
  public_id: string;
  org_id: string;
  employee_id: string;
  warning_type_id: string;
  warning_type_name: string;
  severity_level: number;
  title: string;
  description: string;
  incident_date: string;
  issued_by: string;
  witness_ids: string[];
  can_employee_respond: boolean;
  response_window_days: number;
  response_deadline?: string;
  employee_response?: string;
  appeal_reason?: string;
  appeal_resolution?: string;
  expires_at?: string;
  is_active: boolean;
  issued_at?: string;
  approval_instance_id?: string;
  status: WarningStatus;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface WarningListResponse {
  warnings: EmployeeWarning[];
  total: number;
}

export interface CreateWarningPayload {
  warning_type_id: string;
  title: string;
  description: string;
  incident_date: string;
  witness_ids?: string[];
}

export interface AppealWarningPayload {
  reason: string;
}

export interface CloseWarningPayload {
  appeal_resolution?: string;
}

// ── Setup: Shifts ─────────────────────────────────────────
export type ShiftType = "fixed" | "flexible";
export type ScheduleAssigneeType = "organization" | "department" | "employee";

export interface Shift {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  shift_type: ShiftType;
  start_time?: string;
  end_time?: string;
  core_start_time?: string;
  core_end_time?: string;
  weekly_hours_target?: number;
  break_minutes: number;
  working_days: string[];
  track_overtime: boolean;
  overtime_threshold_hours?: number;
  track_breaks: boolean;
  is_default: boolean;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ShiftListResponse {
  shifts: Shift[];
  total: number;
}

export interface CreateShiftPayload {
  name: string;
  description?: string;
  shift_type: ShiftType;
  start_time?: string;
  end_time?: string;
  core_start_time?: string;
  core_end_time?: string;
  weekly_hours_target?: number;
  break_minutes?: number;
  working_days?: string[];
  track_overtime?: boolean;
  overtime_threshold_hours?: number;
  track_breaks?: boolean;
  is_default?: boolean;
}

export interface UpdateShiftPayload extends Partial<CreateShiftPayload> {
  is_active?: boolean;
}

export interface WorkScheduleAssignment {
  id: string;
  public_id: string;
  org_id: string;
  shift_id: string;
  assignee_type: ScheduleAssigneeType;
  assignee_id: string;
  effective_date: string;
  end_date?: string;
  created_by: string;
  created_at: string;
}

export interface AssignmentListResponse {
  assignments: WorkScheduleAssignment[];
  total: number;
}

export interface AssignShiftPayload {
  shift_id: string;
  assignee_type: ScheduleAssigneeType;
  assignee_id: string;
  effective_date: string;
  end_date?: string;
}

// ── Setup: Holiday Calendars ──────────────────────────────
export type HolidayType = "public" | "company" | "optional";

export interface HolidayCalendar {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  country_code?: string;
  year: number;
  is_active: boolean;
  holidays?: Holiday[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CalendarListResponse {
  calendars: HolidayCalendar[];
  total: number;
}

export interface CreateCalendarPayload {
  name: string;
  description?: string;
  country_code?: string;
  year: number;
}

export interface Holiday {
  id: string;
  public_id: string;
  calendar_id: string;
  name: string;
  date: string;
  holiday_type: HolidayType;
  is_paid: boolean;
  repeat_yearly: boolean;
  created_at: string;
}

export interface HolidayListResponse {
  holidays: Holiday[];
  total: number;
}

export interface CreateHolidayPayload {
  name: string;
  date: string;
  holiday_type: HolidayType;
  is_paid?: boolean;
  repeat_yearly?: boolean;
}

export interface CalendarAssignment {
  id: string;
  public_id: string;
  org_id: string;
  calendar_id: string;
  assignee_type: ScheduleAssigneeType;
  assignee_id: string;
  effective_date: string;
  created_by: string;
  created_at: string;
}

export interface AssignCalendarPayload {
  calendar_id: string;
  assignee_type: ScheduleAssigneeType;
  assignee_id: string;
  effective_date: string;
}
// ── Setup: Document Templates ─────────────────────────────
export type DocumentTemplateType =
  | "offer_letter"
  | "contract"
  | "warning_letter"
  | "promotion_letter"
  | "transfer_letter"
  | "termination_letter"
  | "resignation_acceptance"
  | "experience_letter"
  | "nda"
  | "policy"
  | "custom";

export interface DocumentTemplate {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  document_type: DocumentTemplateType;
  description?: string;
  body_markdown: string;
  available_variables: string[];
  requires_acknowledgement: boolean;
  is_active: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface DocumentTemplateListResponse {
  templates: DocumentTemplate[];
  total: number;
}

export interface CreateDocumentTemplatePayload {
  name: string;
  document_type: DocumentTemplateType;
  description?: string;
  body_markdown: string;
  available_variables?: string[];
  requires_acknowledgement?: boolean;
}

export interface UpdateDocumentTemplatePayload extends Partial<CreateDocumentTemplatePayload> {
  is_active?: boolean;
}

export interface PreviewTemplateResult {
  filled_content: string;
  variables_used: string[];
  missing?: string[];
}

// ── Setup: Approval Chains ────────────────────────────────
export type ApprovalActionType =
  | "leave"
  | "resignation"
  | "promotion"
  | "transfer"
  | "warning"
  | "document"
  | "termination"
  | "attendance_regularization"
  | "custom";
export type ApproverType =
  | "reporting_manager"
  | "dept_head"
  | "role"
  | "specific_user";
export type SLABreachAction = "escalate_next" | "auto_approve" | "auto_reject";
export type ApprovalInstanceStatus =
  | "pending"
  | "approved"
  | "rejected"
  | "cancelled";

export interface ApprovalTemplateLevel {
  id?: string;
  level: number;
  approver_type: ApproverType;
  approver_role?: string;
  approver_user_id?: string;
  sla_hours: number;
  on_sla_breach: SLABreachAction;
}

export interface ApprovalTemplate {
  id: string;
  public_id: string;
  org_id: string;
  name: string;
  description?: string;
  action_type: ApprovalActionType;
  condition_expression?: string;
  is_default: boolean;
  is_active: boolean;
  levels?: ApprovalTemplateLevel[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface TemplateListResponse {
  templates: ApprovalTemplate[];
  total: number;
}

export interface CreateTemplatePayload {
  name: string;
  description?: string;
  action_type: ApprovalActionType;
  condition_expression?: string;
  is_default?: boolean;
  levels: ApprovalTemplateLevel[];
}

export interface UpdateTemplatePayload {
  name?: string;
  description?: string;
  condition_expression?: string;
  is_default?: boolean;
  is_active?: boolean;
}

export interface ApprovalDecision {
  id: string;
  instance_id: string;
  level: number;
  approver_id: string;
  action: string;
  note?: string;
  decided_at: string;
}

export interface ApprovalInstance {
  id: string;
  public_id: string;
  org_id: string;
  template_id?: string;
  entity_type: string;
  entity_id: string;
  current_level: number;
  overall_status: ApprovalInstanceStatus;
  requested_by: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  snapshot?: ApprovalTemplateLevel[];
  decisions?: ApprovalDecision[];
}

export interface DecisionPayload {
  note?: string;
}

// ── Reports ────────────────────────────────────────────────
export interface HRMSummary {
  total_employees: number;
  active_employees: number;
  on_leave_employees: number;
  terminated_employees: number;
  total_departments: number;
  total_positions: number;
  pending_leave_requests: number;
  approved_leave_today: number;
}

export interface HeadcountByDepartment {
  department_id: string;
  department_name: string;
  headcount: number;
}

export interface LeaveSummaryByType {
  leave_type_id: string;
  leave_type_name: string;
  total_requests: number;
  approved: number;
  pending: number;
  rejected: number;
  total_days: number;
}
