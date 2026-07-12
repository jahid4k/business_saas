// src/lib/hrm/salary.ts
import api from "../api";
import type {
  SalaryComponent,
  SalaryComponentListResponse,
  CreateSalaryComponentPayload,
  UpdateSalaryComponentPayload,
  SalaryStructure,
  SalaryStructureListResponse,
  CreateSalaryStructurePayload,
  AddComponentToStructurePayload,
  EmployeeSalaryRecord,
  SalaryHistoryResponse,
  AssignSalaryPayload,
  TestFormulaPayload,
  TestFormulaResult,
} from "@/types/hrm";

// ── Components ────────────────────────────────────────────
const componentsUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/salary/components`;

export async function listSalaryComponents(
  orgId: string,
): Promise<SalaryComponentListResponse> {
  const res = await api.get<{
    success: boolean;
    data: SalaryComponentListResponse;
  }>(componentsUrl(orgId));
  return res.data.data;
}

export async function createSalaryComponent(
  orgId: string,
  body: CreateSalaryComponentPayload,
): Promise<SalaryComponent> {
  const res = await api.post<{
    success: boolean;
    data: { component: SalaryComponent };
  }>(componentsUrl(orgId), body);
  return res.data.data.component;
}

export async function updateSalaryComponent(
  orgId: string,
  compId: string,
  body: UpdateSalaryComponentPayload,
): Promise<SalaryComponent> {
  const res = await api.patch<{
    success: boolean;
    data: { component: SalaryComponent };
  }>(`${componentsUrl(orgId)}/${compId}`, body);
  return res.data.data.component;
}

export async function deleteSalaryComponent(
  orgId: string,
  compId: string,
): Promise<void> {
  await api.delete(`${componentsUrl(orgId)}/${compId}`);
}

export async function testSalaryFormula(
  orgId: string,
  body: TestFormulaPayload,
): Promise<TestFormulaResult> {
  const res = await api.post<{
    success: boolean;
    data: { formula_test: TestFormulaResult };
  }>(`/api/v1/organizations/${orgId}/hrm/setup/salary/formula/test`, body);
  return res.data.data.formula_test;
}

// ── Structures ────────────────────────────────────────────
const structuresUrl = (orgId: string) =>
  `/api/v1/organizations/${orgId}/hrm/setup/salary/structures`;

export async function listSalaryStructures(
  orgId: string,
): Promise<SalaryStructureListResponse> {
  const res = await api.get<{
    success: boolean;
    data: SalaryStructureListResponse;
  }>(structuresUrl(orgId));
  return res.data.data;
}

export async function getSalaryStructure(
  orgId: string,
  structId: string,
): Promise<SalaryStructure> {
  const res = await api.get<{
    success: boolean;
    data: { structure: SalaryStructure };
  }>(`${structuresUrl(orgId)}/${structId}`);
  return res.data.data.structure;
}

export async function createSalaryStructure(
  orgId: string,
  body: CreateSalaryStructurePayload,
): Promise<SalaryStructure> {
  const res = await api.post<{
    success: boolean;
    data: { structure: SalaryStructure };
  }>(structuresUrl(orgId), body);
  return res.data.data.structure;
}

export async function deleteSalaryStructure(
  orgId: string,
  structId: string,
): Promise<void> {
  await api.delete(`${structuresUrl(orgId)}/${structId}`);
}

export async function addComponentToStructure(
  orgId: string,
  structId: string,
  body: AddComponentToStructurePayload,
): Promise<void> {
  await api.post(`${structuresUrl(orgId)}/${structId}/components`, body);
}

export async function removeComponentFromStructure(
  orgId: string,
  structId: string,
  compId: string,
): Promise<void> {
  await api.delete(`${structuresUrl(orgId)}/${structId}/components/${compId}`);
}

// ── Employee salary ───────────────────────────────────────
const empSalaryUrl = (orgId: string, employeeId: string) =>
  `/api/v1/organizations/${orgId}/hrm/employees/${employeeId}/salary`;

export async function getEmployeeSalaryHistory(
  orgId: string,
  employeeId: string,
): Promise<SalaryHistoryResponse> {
  const res = await api.get<{ success: boolean; data: SalaryHistoryResponse }>(
    empSalaryUrl(orgId, employeeId),
  );
  return res.data.data;
}

export async function assignEmployeeSalary(
  orgId: string,
  employeeId: string,
  body: AssignSalaryPayload,
): Promise<EmployeeSalaryRecord> {
  const res = await api.post<{
    success: boolean;
    data: { salary_record: EmployeeSalaryRecord };
  }>(empSalaryUrl(orgId, employeeId), body);
  return res.data.data.salary_record;
}

// Records come back newest-first; the first one with effective_date <= today is active.
export function activeSalaryRecord(
  records: EmployeeSalaryRecord[],
): EmployeeSalaryRecord | undefined {
  const today = new Date().toISOString().slice(0, 10);
  return records.find((r) => r.effective_date <= today) ?? records[0];
}
