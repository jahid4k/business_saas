// src/components/hrm/doctemplates/TemplatePreview.tsx
"use client";

import { useState } from "react";
import { Eye } from "lucide-react";
import { previewDocumentTemplate } from "@/lib/hrm/doctemplates";
import type { DocumentTemplate, PreviewTemplateResult } from "@/types/hrm";

interface TemplatePreviewProps {
  orgId: string;
  template: DocumentTemplate;
}

const inputCls = `
  w-full px-3.5 py-2.5 rounded-lg text-sm
  bg-[var(--bg-elevated)] border border-[var(--border)]
  text-[var(--text-primary)] placeholder:text-[var(--text-muted)]
  outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/15
  transition-all
`;

export default function TemplatePreview({
  orgId,
  template,
}: TemplatePreviewProps) {
  const [values, setValues] = useState<Record<string, string>>(
    Object.fromEntries(template.available_variables.map((v) => [v, ""])),
  );
  const [result, setResult] = useState<PreviewTemplateResult | null>(null);
  const [loading, setLoading] = useState(false);

  const handlePreview = async () => {
    setLoading(true);
    try {
      const r = await previewDocumentTemplate(orgId, template.id, values);
      setResult(r);
    } catch {
      setResult({
        filled_content: "Failed to generate preview.",
        variables_used: [],
      });
    }
    setLoading(false);
  };

  return (
    <div className="px-6 py-5 space-y-4">
      {template.available_variables.length > 0 && (
        <div className="space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
            Fill sample values
          </p>
          {template.available_variables.map((v) => (
            <div key={v} className="space-y-1">
              <label className="block text-xs font-medium text-[var(--text-secondary)]">
                {v}
              </label>
              <input
                value={values[v] ?? ""}
                onChange={(e) =>
                  setValues((old) => ({ ...old, [v]: e.target.value }))
                }
                placeholder={`Value for ${v}`}
                className={inputCls}
              />
            </div>
          ))}
        </div>
      )}

      <button
        onClick={handlePreview}
        disabled={loading}
        className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 transition-colors"
      >
        <Eye size={15} />
        {loading ? "Generating…" : "Generate preview"}
      </button>

      {result && (
        <div className="space-y-2">
          {result.missing && result.missing.length > 0 && (
            <p className="text-xs text-amber-400">
              Missing values for: {result.missing.join(", ")}
            </p>
          )}
          <div className="p-4 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border)] whitespace-pre-wrap text-sm text-[var(--text-primary)] max-h-80 overflow-y-auto">
            {result.filled_content}
          </div>
        </div>
      )}
    </div>
  );
}
