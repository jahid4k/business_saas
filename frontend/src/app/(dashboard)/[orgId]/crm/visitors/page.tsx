"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Globe, Code, Loader2, Copy, CheckCircle2, UserCheck } from "lucide-react";
import { useDrawer } from "@/contexts/DrawerContext";
import { listVisitors } from "@/lib/visitors";
import { listAPIKeys } from "@/lib/apikeys";

function SnippetModal({ orgId }: { orgId: string }) {
  const { closeDrawer } = useDrawer();
  const [copied, setCopied] = useState(false);
  const { data: keys, isLoading } = useQuery({
    queryKey: ["apikeys", orgId],
    queryFn: () => listAPIKeys(orgId),
  });

  // Try to find a key with capture:visitors scope, or fallback to the first one, or "YOUR_API_KEY"
  const visitorKey = keys?.find((k) => k.scopes.includes("capture:visitors"))?.key_prefix || keys?.[0]?.key_prefix || "YOUR_API_KEY_HERE";

  const snippet = `<script>
  (function(w,d,s,l,i){
    w[l]=w[l]||function(){(w[l].q=w[l].q||[]).push(arguments)};
    var f=d.getElementsByTagName(s)[0],j=d.createElement(s);
    j.async=true;j.src='https://api.businesssaas.com/tracker.js';
    f.parentNode.insertBefore(j,f);
  })(window,document,'script','bsaas');

  bsaas('init', '${visitorKey}');
  bsaas('page');
</script>`;

  const copyToClipboard = () => {
    navigator.clipboard.writeText(snippet);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="p-6 space-y-6">
      <div className="mb-2">
        <p className="text-sm text-[var(--text-muted)]">
          Copy and paste this snippet into the <code>&lt;head&gt;</code> of your website to start tracking visitors.
        </p>
      </div>

      <div className="relative">
        <pre className="p-4 bg-[var(--bg-surface)] border border-[var(--border)] rounded-xl text-xs overflow-x-auto font-mono text-[var(--text-secondary)]">
          {isLoading ? "Loading API key..." : snippet}
        </pre>
        <button
          onClick={copyToClipboard}
          className="absolute top-3 right-3 p-1.5 bg-[var(--bg-elevated)] border border-[var(--border)] rounded-md hover:bg-black/5 transition-colors"
        >
          {copied ? <CheckCircle2 size={14} className="text-green-500" /> : <Copy size={14} className="text-[var(--text-muted)]" />}
        </button>
      </div>

      <div className="flex justify-end pt-4 border-t border-[var(--border)]">
        <button
          onClick={() => closeDrawer()}
          className="px-4 py-2 rounded-lg text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          Close
        </button>
      </div>
    </div>
  );
}

export default function VisitorsPage() {
  const params = useParams();
  const orgId = params.orgId as string;
  const { openDrawer } = useDrawer();

  const { data: visitors, isLoading } = useQuery({
    queryKey: ["visitors", orgId],
    queryFn: () => listVisitors(orgId),
    refetchInterval: 30000, // Refresh every 30s
  });

  const handleShowSnippet = () => {
    openDrawer({
      title: "Tracking Snippet",
      width: "md",
      content: <SnippetModal orgId={orgId} />,
    });
  };

  return (
    <div className="p-6 md:p-8 max-w-6xl space-y-8">
      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
        <div>
          <h1
            className="text-2xl font-bold text-[var(--text-primary)] mb-1"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)", letterSpacing: "-0.02em" }}
          >
            Website Visitors
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Track who is browsing your website and automatically identify companies.
          </p>
        </div>
        <button
          onClick={handleShowSnippet}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
        >
          <Code size={16} />
          View Embed Code
        </button>
      </div>

      {isLoading ? (
        <div className="py-20 flex flex-col items-center justify-center text-sm text-[var(--text-muted)]">
          <Loader2 size={24} className="animate-spin mb-4 text-purple-500" />
          Loading visitor data...
        </div>
      ) : !visitors || visitors.length === 0 ? (
        <div className="py-20 text-center border border-dashed border-[var(--border)] rounded-2xl bg-[var(--bg-surface)]">
          <Globe size={40} className="mx-auto text-[var(--text-muted)] mb-4 opacity-50" />
          <h3 className="text-lg font-semibold text-[var(--text-primary)]">No visitors yet</h3>
          <p className="text-sm text-[var(--text-muted)] mt-1 max-w-sm mx-auto">
            Embed the tracking snippet on your website to start identifying companies visiting your site.
          </p>
          <button
            onClick={handleShowSnippet}
            className="mt-6 px-4 py-2 rounded-lg text-sm font-medium text-[var(--text-primary)] border border-[var(--border)] hover:bg-[var(--bg-elevated)] transition-colors"
          >
            Get Snippet
          </button>
        </div>
      ) : (
        <div className="border border-[var(--border)] rounded-2xl overflow-hidden bg-[var(--bg-surface)]">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-[var(--bg-elevated)] border-b border-[var(--border)] text-[var(--text-muted)] font-medium">
                <tr>
                  <th className="px-6 py-4">Visitor / Company</th>
                  <th className="px-6 py-4">IP Address</th>
                  <th className="px-6 py-4">Session</th>
                  <th className="px-6 py-4">Last Active</th>
                  <th className="px-6 py-4 text-right">CRM Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border)] text-[var(--text-secondary)]">
                {visitors.map((v) => (
                  <tr key={v.id} className="hover:bg-[var(--bg-elevated)] transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-full bg-blue-500/10 text-blue-500 flex items-center justify-center border border-blue-500/20">
                          <Globe size={14} />
                        </div>
                        <div>
                          <p className="font-medium text-[var(--text-primary)]">
                            {v.company_name || "Unknown Company"}
                          </p>
                          {v.company_domain && (
                            <p className="text-xs text-[var(--text-muted)]">{v.company_domain}</p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4 font-mono text-xs">{v.ip_address || "—"}</td>
                    <td className="px-6 py-4 font-mono text-xs text-[var(--text-muted)]">
                      {v.session_id.substring(0, 8)}...
                    </td>
                    <td className="px-6 py-4">
                      {new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: 'numeric' }).format(new Date(v.updated_at))}
                    </td>
                    <td className="px-6 py-4 text-right">
                      {v.linked_lead_id ? (
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-green-500/10 text-green-600 border border-green-500/20">
                          <UserCheck size={12} />
                          Captured
                        </span>
                      ) : (
                        <span className="text-xs text-[var(--text-muted)]">Anonymous</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
