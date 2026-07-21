// src/components/crm/setup/TemplateForm.tsx
"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import type { TemplateModel, TemplateType } from "@/lib/crm/templates";

interface TemplateFormProps {
  initialData?: TemplateModel;
  onSave: (payload: { name: string; type: TemplateType; subject?: string; body: string }) => Promise<void>;
  onCancel: () => void;
}

export default function TemplateForm({ initialData, onSave, onCancel }: TemplateFormProps) {
  const [loading, setLoading] = useState(false);
  
  const [name, setName] = useState(initialData?.name || "");
  const [type, setType] = useState<TemplateType>(initialData?.type || "email");
  const [subject, setSubject] = useState(initialData?.subject || "");
  const [body, setBody] = useState(initialData?.body || "");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !body.trim()) return;
    if (type === "email" && !subject.trim()) return;

    setLoading(true);
    try {
      await onSave({
        name: name.trim(),
        type,
        subject: type === "email" ? subject.trim() : undefined,
        body: body.trim(),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        <div>
          <label className="block text-sm font-medium text-(--text-secondary) mb-1.5">
            Template Name
          </label>
          <input
            type="text"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Initial Outreach"
            className="w-full px-3.5 py-2.5 bg-(--bg-surface) border border-(--border) rounded-lg text-(--text-primary) focus:outline-none focus:border-purple-500 transition-colors"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-(--text-secondary) mb-1.5">
            Template Type
          </label>
          <div className="flex gap-4">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="type"
                value="email"
                checked={type === "email"}
                onChange={() => setType("email")}
                disabled={!!initialData} // don't allow changing type after creation
                className="accent-purple-500"
              />
              <span className="text-sm text-(--text-primary)">Email</span>
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                name="type"
                value="note"
                checked={type === "note"}
                onChange={() => setType("note")}
                disabled={!!initialData}
                className="accent-purple-500"
              />
              <span className="text-sm text-(--text-primary)">Note</span>
            </label>
          </div>
        </div>

        {type === "email" && (
          <div>
            <label className="block text-sm font-medium text-(--text-secondary) mb-1.5">
              Subject Line
            </label>
            <input
              type="text"
              required={type === "email"}
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="e.g. Following up on our conversation"
              className="w-full px-3.5 py-2.5 bg-(--bg-surface) border border-(--border) rounded-lg text-(--text-primary) focus:outline-none focus:border-purple-500 transition-colors"
            />
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-(--text-secondary) mb-1.5">
            Body Content
          </label>
          <textarea
            required
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Hi {{first_name}},\n\n..."
            rows={8}
            className="w-full px-3.5 py-2.5 bg-(--bg-surface) border border-(--border) rounded-lg text-(--text-primary) focus:outline-none focus:border-purple-500 transition-colors resize-none"
          />
          <p className="text-xs text-(--text-muted) mt-1.5">
            You can use placeholders like {"{{first_name}}"}, {"{{company_name}}"}, etc.
          </p>
        </div>
      </div>

      <div className="p-4 border-t border-(--border) bg-(--bg-canvas) flex justify-end gap-3 shrink-0 rounded-b-xl">
        <button
          type="button"
          onClick={onCancel}
          disabled={loading}
          className="px-4 py-2 text-sm font-medium text-(--text-secondary) hover:text-(--text-primary) hover:bg-(--bg-surface) border border-transparent hover:border-(--border) rounded-lg transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={loading || !name.trim() || !body.trim() || (type === "email" && !subject.trim())}
          className="px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg shadow-sm transition-all flex items-center gap-2"
        >
          {loading && <Loader2 size={16} className="animate-spin" />}
          {initialData ? "Save Changes" : "Create Template"}
        </button>
      </div>
    </form>
  );
}
// Let me check existing forms in the project.
