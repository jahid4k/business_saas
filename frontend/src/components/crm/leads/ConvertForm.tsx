// src/components/crm/leads/ConvertForm.tsx
"use client";

import { useEffect, useState } from "react";
import { useDrawer } from "@/contexts/DrawerContext";
import { listPipelines, listStages } from "@/lib/crm/pipelines";
import type { Lead, Pipeline, Stage } from "@/types/crm";

interface ConvertFormProps {
  lead: Lead;
  orgId: string;
  onSave: (payload: {
    create_contact: boolean;
    create_deal: boolean;
    deal_title?: string;
    pipeline_id?: string;
    stage_id?: string;
    deal_value?: number;
  }) => Promise<void>;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function ConvertForm({ lead, orgId, onSave }: ConvertFormProps) {
  const { closeDrawer } = useDrawer();

  // Form state
  const [createContact, setCreateContact] = useState(true);
  const [createDeal, setCreateDeal] = useState(true);
  const [dealTitle, setDealTitle] = useState(
    [lead.first_name, lead.last_name].filter(Boolean).join(" ") +
      (lead.company_name ? ` — ${lead.company_name}` : ""),
  );
  const [dealValue, setDealValue] = useState("");
  const [pipelineId, setPipelineId] = useState("");
  const [stageId, setStageId] = useState("");

  // Pipeline + stage data
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [stages, setStages] = useState<Stage[]>([]);
  const [loadingPipes, setLoadingPipes] = useState(true);
  const [loadingStages, setLoadingStages] = useState(false);

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch pipelines on mount
  useEffect(() => {
    listPipelines(orgId)
      .then((p) => {
        setPipelines(p);
        // Auto-select default pipeline
        const def = p.find((x) => x.is_default) ?? p[0];
        if (def) setPipelineId(def.id);
      })
      .finally(() => setLoadingPipes(false));
  }, [orgId]);

  // Fetch stages when pipeline changes
  useEffect(() => {
    if (!pipelineId) {
      setStages([]);
      setStageId("");
      return;
    }
    setLoadingStages(true);
    setStageId("");
    listStages(orgId, pipelineId)
      .then((s) => {
        setStages(s);
        if (s.length > 0) setStageId(s[0].id);
      })
      .finally(() => setLoadingStages(false));
  }, [pipelineId, orgId]);

  const handleSave = async () => {
    setError(null);
    setSaving(true);
    try {
      await onSave({
        create_contact: createContact,
        create_deal: createDeal && createContact,
        deal_title: createDeal ? dealTitle || undefined : undefined,
        pipeline_id: createDeal && pipelineId ? pipelineId : undefined,
        stage_id: createDeal && stageId ? stageId : undefined,
        deal_value: createDeal && dealValue ? parseFloat(dealValue) : undefined,
      });
      closeDrawer();
    } catch {
      setError("Failed to convert lead. Please try again.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
        {/* Lead summary */}
        <div className="flex items-center gap-3 p-4 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)]">
          <div
            className="w-9 h-9 rounded-full flex-shrink-0 flex items-center justify-center text-sm font-bold text-white"
            style={{ background: "linear-gradient(135deg, #7c3aed, #a855f7)" }}
          >
            {lead.first_name[0].toUpperCase()}
          </div>
          <div>
            <p className="text-sm font-medium text-[var(--text-primary)]">
              {lead.first_name} {lead.last_name}
            </p>
            <p className="text-xs text-[var(--text-muted)]">
              {lead.company_name ?? lead.email ?? "No company"}
            </p>
          </div>
        </div>

        {error && (
          <div className="px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/10 border border-red-500/20">
            {error}
          </div>
        )}

        {/* ── Create contact ── */}
        <div
          className={`rounded-xl border transition-colors ${createContact ? "border-purple-500/30 bg-purple-500/5" : "border-[var(--border)]"}`}
        >
          <label className="flex items-start gap-3 p-4 cursor-pointer">
            <input
              type="checkbox"
              checked={createContact}
              onChange={(e) => {
                setCreateContact(e.target.checked);
                if (!e.target.checked) setCreateDeal(false);
              }}
              className="mt-0.5 accent-purple-500 flex-shrink-0"
            />
            <div>
              <p className="text-sm font-medium text-[var(--text-primary)]">
                Create contact
              </p>
              <p className="text-xs text-[var(--text-muted)] mt-0.5">
                Add {lead.first_name} {lead.last_name} to your contacts
              </p>
            </div>
          </label>
        </div>

        {/* ── Create deal ── */}
        <div
          className={`rounded-xl border transition-all ${
            !createContact
              ? "opacity-40 pointer-events-none border-[var(--border)]"
              : createDeal
                ? "border-purple-500/30 bg-purple-500/5"
                : "border-[var(--border)]"
          }`}
        >
          <label className="flex items-start gap-3 p-4 cursor-pointer">
            <input
              type="checkbox"
              checked={createDeal}
              onChange={(e) => setCreateDeal(e.target.checked)}
              disabled={!createContact}
              className="mt-0.5 accent-purple-500 flex-shrink-0"
            />
            <div>
              <p className="text-sm font-medium text-[var(--text-primary)]">
                Create deal
              </p>
              <p className="text-xs text-[var(--text-muted)] mt-0.5">
                Add an opportunity to your pipeline
              </p>
            </div>
          </label>

          {/* Deal fields — shown when createDeal is true */}
          {createDeal && createContact && (
            <div className="px-4 pb-4 space-y-3 border-t border-[var(--border)] pt-3">
              {/* Deal title */}
              <div className="space-y-1.5">
                <label className="block text-xs font-medium text-[var(--text-secondary)]">
                  Deal title
                </label>
                <input
                  value={dealTitle}
                  onChange={(e) => setDealTitle(e.target.value)}
                  placeholder="Deal title"
                  className={inputCls}
                />
              </div>

              {/* Deal value */}
              <div className="space-y-1.5">
                <label className="block text-xs font-medium text-[var(--text-secondary)]">
                  Deal value (optional)
                </label>
                <div className="relative">
                  <span className="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm text-[var(--text-muted)]">
                    $
                  </span>
                  <input
                    type="number"
                    value={dealValue}
                    onChange={(e) => setDealValue(e.target.value)}
                    placeholder="0"
                    min="0"
                    className={`${inputCls} pl-7`}
                  />
                </div>
              </div>

              {/* Pipeline */}
              <div className="space-y-1.5">
                <label className="block text-xs font-medium text-[var(--text-secondary)]">
                  Pipeline (optional)
                </label>
                {loadingPipes ? (
                  <p className="text-xs text-[var(--text-muted)] py-2">
                    Loading pipelines…
                  </p>
                ) : pipelines.length === 0 ? (
                  <p className="text-xs text-[var(--text-muted)] py-2">
                    No pipelines found
                  </p>
                ) : (
                  <select
                    value={pipelineId}
                    onChange={(e) => setPipelineId(e.target.value)}
                    className={inputCls}
                  >
                    <option value="">No pipeline</option>
                    {pipelines.map((p) => (
                      <option
                        key={p.id}
                        value={p.id}
                        style={{ background: "var(--bg-elevated)" }}
                      >
                        {p.name}
                        {p.is_default ? " (default)" : ""}
                      </option>
                    ))}
                  </select>
                )}
              </div>

              {/* Stage */}
              {pipelineId && (
                <div className="space-y-1.5">
                  <label className="block text-xs font-medium text-[var(--text-secondary)]">
                    Stage (optional)
                  </label>
                  {loadingStages ? (
                    <p className="text-xs text-[var(--text-muted)] py-2">
                      Loading stages…
                    </p>
                  ) : (
                    <select
                      value={stageId}
                      onChange={(e) => setStageId(e.target.value)}
                      className={inputCls}
                    >
                      <option value="">No stage</option>
                      {stages.map((s) => (
                        <option
                          key={s.id}
                          value={s.id}
                          style={{ background: "var(--bg-elevated)" }}
                        >
                          {s.name}
                        </option>
                      ))}
                    </select>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="flex items-center gap-3 px-6 py-4 border-t border-[var(--border)] flex-shrink-0">
        <button
          type="button"
          onClick={closeDrawer}
          className="flex-1 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Cancel
        </button>
        <button
          onClick={handleSave}
          disabled={saving || (!createContact && !createDeal)}
          className="flex-1 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? "Converting…" : "Convert lead"}
        </button>
      </div>
    </div>
  );
}
