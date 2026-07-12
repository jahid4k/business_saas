// src/components/hrm/salary/SalaryComponentForm.tsx
"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { FlaskConical } from "lucide-react";
import { useDrawer } from "@/contexts/DrawerContext";
import { testSalaryFormula } from "@/lib/hrm/salary";
import type {
  SalaryComponent,
  CreateSalaryComponentPayload,
} from "@/types/hrm";

const COMPONENT_TYPES = [
  { value: "earning", label: "Earning" },
  { value: "deduction", label: "Deduction" },
  { value: "employer_contribution", label: "Employer contribution" },
];

const CALC_METHODS = [
  { value: "fixed", label: "Fixed amount" },
  { value: "pct_of_basic", label: "% of basic pay" },
  { value: "pct_of_gross", label: "% of gross pay" },
  { value: "formula", label: "Formula" },
  { value: "manual", label: "Manual (entered per payslip)" },
  { value: "slab", label: "Slab / bracket (not yet computed by payroll)" },
];

const optionalNumber = z.preprocess(
  (v) => (v === "" || v === undefined || v === null ? undefined : v),
  z.coerce.number().optional(),
);

const schema = z.object({
  name: z.string().min(1, "Name is required").max(150, "Max 150 characters"),
  description: z.string().optional(),
  component_type: z.string().min(1, "Type is required"),
  calc_method: z.string().min(1, "Calculation method is required"),
  fixed_value: optionalNumber,
  formula_expression: z.string().optional(),
  slab_config_json: z.string().optional(),
  is_taxable: z.boolean().optional(),
});
type SalaryComponentFormInput = z.input<typeof schema>;
type SalaryComponentFormValues = z.infer<typeof schema>;

interface SalaryComponentFormProps {
  orgId: string;
  component?: SalaryComponent | null;
  onSave: (payload: CreateSalaryComponentPayload) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function SalaryComponentForm({
  orgId,
  component,
  onSave,
}: SalaryComponentFormProps) {
  const { closeDrawer } = useDrawer();
  const [error, setError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  const isEdit = !!component;

  const {
    register,
    handleSubmit,
    watch,
    getValues,
    formState: { errors, isSubmitting },
  } = useForm<SalaryComponentFormInput, undefined, SalaryComponentFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: component?.name ?? "",
      description: component?.description ?? "",
      component_type: component?.component_type ?? "earning",
      calc_method: component?.calc_method ?? "fixed",
      fixed_value: component?.fixed_value ?? "",
      formula_expression: component?.formula_expression ?? "",
      slab_config_json: component?.slab_config
        ? JSON.stringify(component.slab_config, null, 2)
        : "",
      is_taxable: component?.is_taxable ?? true,
    },
  });

  const calcMethod = watch("calc_method");

  const handleTestFormula = async () => {
    const expr = getValues("formula_expression");
    if (!expr) return;
    setTesting(true);
    setTestResult(null);
    try {
      const result = await testSalaryFormula(orgId, {
        expression: expr,
        variables: {
          BASIC: 50000,
          GROSS: 60000,
          PRESENT_DAYS: 22,
          WORK_DAYS: 22,
          TENURE_YEARS: 2,
        },
      });
      setTestResult(
        result.valid
          ? `✓ Valid — result: ${result.result} (using sample BASIC=50000, GROSS=60000, PRESENT_DAYS=22, WORK_DAYS=22, TENURE_YEARS=2)`
          : `✗ ${result.error ?? "Invalid formula"}`,
      );
    } catch {
      setTestResult("✗ Failed to test formula.");
    }
    setTesting(false);
  };

  const onSubmit = async (values: SalaryComponentFormValues) => {
    setError(null);
    let slabConfig;
    if (values.calc_method === "slab" && values.slab_config_json) {
      try {
        slabConfig = JSON.parse(values.slab_config_json);
      } catch {
        setError(
          'Slab config must be valid JSON, e.g. {"base_variable":"GROSS","slabs":[{"up_to":30000,"rate":0.05},{"up_to":null,"rate":0.1}]}',
        );
        return;
      }
    }
    try {
      await onSave({
        name: values.name,
        description: values.description || undefined,
        component_type:
          values.component_type as CreateSalaryComponentPayload["component_type"],
        calc_method:
          values.calc_method as CreateSalaryComponentPayload["calc_method"],
        fixed_value: values.fixed_value,
        formula_expression:
          values.calc_method === "formula"
            ? values.formula_expression || undefined
            : undefined,
        slab_config: slabConfig,
        is_taxable: values.is_taxable,
      });
      closeDrawer();
    } catch {
      setError("Failed to save salary component. Please try again.");
    }
  };

  return (
    <div className="flex flex-col h-full">
      <form
        id="salary-component-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="flex-1 overflow-y-auto px-6 py-5 space-y-4"
      >
        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Name <span className="text-red-400">*</span>
          </label>
          <input
            {...register("name")}
            autoFocus
            placeholder="e.g. Basic Pay, House Rent, Tax"
            className={inputCls}
          />
          {errors.name && (
            <p className="text-xs text-red-400">{errors.name.message}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Description
          </label>
          <input
            {...register("description")}
            placeholder="Optional"
            className={inputCls}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Type
            </label>
            <select {...register("component_type")} className={inputCls}>
              {COMPONENT_TYPES.map((t) => (
                <option
                  key={t.value}
                  value={t.value}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {t.label}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-center gap-2.5 pt-6">
            <input
              type="checkbox"
              id="is_taxable"
              {...register("is_taxable")}
              className="w-4 h-4 accent-purple-600"
            />
            <label
              htmlFor="is_taxable"
              className="text-sm text-[var(--text-secondary)]"
            >
              Taxable
            </label>
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="block text-sm font-medium text-[var(--text-secondary)]">
            Calculation method
          </label>
          <select {...register("calc_method")} className={inputCls}>
            {CALC_METHODS.map((m) => (
              <option
                key={m.value}
                value={m.value}
                style={{ background: "var(--bg-elevated)" }}
              >
                {m.label}
              </option>
            ))}
          </select>
        </div>

        {(calcMethod === "fixed" ||
          calcMethod === "pct_of_basic" ||
          calcMethod === "pct_of_gross") && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              {calcMethod === "fixed" ? "Fixed amount" : "Percentage"}
            </label>
            <input
              {...register("fixed_value")}
              type="number"
              step="0.01"
              placeholder={
                calcMethod === "fixed" ? "e.g. 5000" : "e.g. 40 (for 40%)"
              }
              className={inputCls}
            />
          </div>
        )}

        {calcMethod === "formula" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Formula expression
            </label>
            <input
              {...register("formula_expression")}
              placeholder="e.g. BASIC * 0.40"
              className={`${inputCls} font-mono`}
            />
            <p className="text-xs text-[var(--text-muted)]">
              Available variables: BASIC, GROSS, PRESENT_DAYS, WORK_DAYS,
              TENURE_YEARS
            </p>
            <button
              type="button"
              onClick={handleTestFormula}
              disabled={testing}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-purple-400 border border-purple-500/30 hover:bg-purple-500/10 disabled:opacity-60 transition-colors"
            >
              <FlaskConical size={13} />
              {testing ? "Testing…" : "Test formula"}
            </button>
            {testResult && (
              <p
                className={`text-xs ${testResult.startsWith("✓") ? "text-emerald-400" : "text-red-400"}`}
              >
                {testResult}
              </p>
            )}
          </div>
        )}

        {calcMethod === "slab" && (
          <div className="space-y-1.5">
            <label className="block text-sm font-medium text-[var(--text-secondary)]">
              Slab config (JSON)
            </label>
            <textarea
              {...register("slab_config_json")}
              rows={5}
              placeholder={
                '{"base_variable":"GROSS","slabs":[{"up_to":30000,"rate":0.05},{"up_to":null,"rate":0.1}]}'
              }
              className={`${inputCls} font-mono`}
            />
            <p className="text-xs text-amber-400">
              Note: the payroll engine doesn&apos;t compute slab amounts yet —
              this will show ৳0 on payslips for now. Configure it now so
              it&apos;s ready once that lands.
            </p>
          </div>
        )}
      </form>

      <div className="flex items-center gap-3 px-6 py-4 border-t border-[var(--border)] flex-shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          form="salary-component-form"
          disabled={isSubmitting}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {isSubmitting
            ? isEdit
              ? "Saving…"
              : "Creating…"
            : isEdit
              ? "Save changes"
              : "Create component"}
        </button>
      </div>
    </div>
  );
}
