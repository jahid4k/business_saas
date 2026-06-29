// src/app/(dashboard)/[orgId]/crm/pipeline/page.tsx
"use client";

import { use, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DndContext,
  DragEndEvent,
  DragOverEvent,
  DragOverlay,
  DragStartEvent,
  PointerSensor,
  useSensor,
  useSensors,
  useDroppable,
  useDraggable,
} from "@dnd-kit/core";
import {
  Plus,
  Trophy,
  X,
  Pencil,
  Building2,
  User,
  Calendar,
  Loader2,
} from "lucide-react";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import { listPipelines, listStages } from "@/lib/crm/pipelines";
import {
  listDeals,
  createDeal,
  updateDeal,
  moveDeal,
  markDealWon,
  markDealLost,
} from "@/lib/crm/deals";
import { listContacts } from "@/lib/crm/contacts";
import { listCompanies } from "@/lib/crm/companies";
import DealForm from "@/components/crm/deals/DealForm";
import type { Deal, Pipeline, Stage, Contact, Company } from "@/types/crm";

// ── Helpers ───────────────────────────────────────────
function formatCurrency(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(value);
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

const STATUS_COLOR: Record<string, string> = {
  open: "text-blue-400",
  won: "text-emerald-400",
  lost: "text-red-400",
};

// ── Deal card ─────────────────────────────────────────
interface DealCardProps {
  deal: Deal;
  contactMap: Map<string, Contact>;
  companyMap: Map<string, Company>;
  onEdit: (deal: Deal) => void;
  onWon: (deal: Deal) => void;
  onLost: (deal: Deal) => void;
  isDragOverlay?: boolean;
}

function DealCard({
  deal,
  contactMap,
  companyMap,
  onEdit,
  onWon,
  onLost,
  isDragOverlay,
}: DealCardProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging } =
    useDraggable({
      id: deal.id,
      data: { fromStageId: deal.stage_id },
    });

  const contact = contactMap.get(deal.contact_id ?? "");
  const company = companyMap.get(deal.company_id ?? "");

  const style: React.CSSProperties = {
    transform: transform
      ? `translate3d(${transform.x}px,${transform.y}px,0)`
      : undefined,
    opacity: isDragging ? 0.35 : 1,
    cursor: isDragOverlay ? "grabbing" : isDragging ? "grabbing" : "grab",
    zIndex: isDragOverlay ? 50 : undefined,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className={`
        rounded-xl p-3.5 border select-none
        bg-[var(--bg-surface)] border-[var(--border)]
        hover:border-purple-500/30 hover:shadow-md
        transition-all duration-150
        ${isDragOverlay ? "shadow-2xl border-purple-500/40 rotate-1" : ""}
      `}
      onClick={(e) => e.stopPropagation()}
    >
      {/* Title + edit */}
      <div className="flex items-start justify-between gap-2 mb-2">
        <p
          className="text-sm font-medium text-[var(--text-primary)] leading-snug"
          style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
        >
          {deal.title}
        </p>
        <button
          onPointerDown={(e) => e.stopPropagation()}
          onClick={(e) => {
            e.stopPropagation();
            onEdit(deal);
          }}
          className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] flex-shrink-0 transition-colors"
        >
          <Pencil size={11} />
        </button>
      </div>

      {/* Value */}
      <p className="text-base font-bold text-[var(--text-primary)] mb-2">
        {formatCurrency(deal.value, deal.currency)}
      </p>

      {/* Company / Contact */}
      <div className="space-y-1 mb-3">
        {company && (
          <div className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
            <Building2 size={10} className="flex-shrink-0" />
            <span className="truncate">{company.name}</span>
          </div>
        )}
        {contact && (
          <div className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
            <User size={10} className="flex-shrink-0" />
            <span className="truncate">
              {contact.first_name} {contact.last_name ?? ""}
            </span>
          </div>
        )}
        {deal.close_date && (
          <div className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
            <Calendar size={10} className="flex-shrink-0" />
            <span>{formatDate(deal.close_date)}</span>
          </div>
        )}
      </div>

      {/* Won / Lost buttons — only for open deals */}
      {deal.status === "open" && !isDragOverlay && (
        <div className="flex items-center gap-1.5">
          <button
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => {
              e.stopPropagation();
              onWon(deal);
            }}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-[0.65rem] font-semibold text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/20 transition-colors"
          >
            <Trophy size={9} />
            Won
          </button>
          <button
            onPointerDown={(e) => e.stopPropagation()}
            onClick={(e) => {
              e.stopPropagation();
              onLost(deal);
            }}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-[0.65rem] font-semibold text-red-400 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 transition-colors"
          >
            <X size={9} />
            Lost
          </button>
        </div>
      )}

      {/* Status badge for won/lost */}
      {deal.status !== "open" && (
        <span
          className={`text-[0.65rem] font-semibold capitalize ${STATUS_COLOR[deal.status]}`}
        >
          {deal.status === "won" ? "✓ Won" : "✗ Lost"}
        </span>
      )}
    </div>
  );
}

// ── Droppable column ──────────────────────────────────
interface ColumnProps {
  stage: Stage;
  deals: Deal[];
  contactMap: Map<string, Contact>;
  companyMap: Map<string, Company>;
  onNewDeal: (stage: Stage) => void;
  onEdit: (deal: Deal) => void;
  onWon: (deal: Deal) => void;
  onLost: (deal: Deal) => void;
  canCreate: boolean;
}

function KanbanColumn({
  stage,
  deals,
  contactMap,
  companyMap,
  onNewDeal,
  onEdit,
  onWon,
  onLost,
  canCreate,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });

  const totalValue = deals
    .filter((d) => d.status === "open")
    .reduce((s, d) => s + d.value, 0);

  return (
    <div className="flex flex-col flex-shrink-0" style={{ width: 272 }}>
      {/* Column header */}
      <div className="flex items-center justify-between px-1 mb-3">
        <div>
          <p
            className="text-sm font-semibold text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
          >
            {stage.name}
          </p>
          <p className="text-xs text-[var(--text-muted)]">
            {deals.length} deal{deals.length !== 1 ? "s" : ""}
            {totalValue > 0 && ` · ${formatCurrency(totalValue)}`}
          </p>
        </div>
        {canCreate && (
          <button
            onClick={() => onNewDeal(stage)}
            className="p-1 rounded-md text-[var(--text-muted)] hover:text-purple-400 hover:bg-purple-500/10 transition-colors"
            title={`Add deal to ${stage.name}`}
          >
            <Plus size={14} />
          </button>
        )}
      </div>

      {/* Drop zone */}
      <div
        ref={setNodeRef}
        className={`
          flex-1 rounded-xl p-2 space-y-2 min-h-[200px]
          transition-colors duration-150
          ${
            isOver
              ? "bg-purple-500/8 border border-purple-500/30"
              : "bg-[var(--bg-elevated)]/50 border border-[var(--border)]/60"
          }
        `}
      >
        {deals.map((deal) => (
          <DealCard
            key={deal.id}
            deal={deal}
            contactMap={contactMap}
            companyMap={companyMap}
            onEdit={onEdit}
            onWon={onWon}
            onLost={onLost}
          />
        ))}
        {deals.length === 0 && (
          <div className="flex items-center justify-center h-24">
            <p className="text-xs text-[var(--text-muted)]">Drop deals here</p>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────
export default function PipelinePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();

  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [stages, setStages] = useState<Stage[]>([]);
  const [deals, setDeals] = useState<Deal[]>([]);
  const [contactMap, setContactMap] = useState(new Map<string, Contact>());
  const [companyMap, setCompanyMap] = useState(new Map<string, Company>());
  const [selectedPipe, setSelectedPipe] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [pageErr, setPageErr] = useState<string | null>(null);
  const [activeDeal, setActiveDeal] = useState<Deal | null>(null); // drag overlay

  const canCreate = hasPermission("crm.deals.create");
  const canUpdate = hasPermission("crm.deals.update");
  const canMoveStage = hasPermission("crm.deals.move_stage");

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
  );

  // Fetch pipelines + contacts + companies on mount
  useEffect(() => {
    Promise.all([
      listPipelines(orgId),
      listContacts(orgId).then((r) => r.contacts),
      listCompanies(orgId).then((r) => r.companies),
    ])
      .then(([pipes, cons, comps]) => {
        setPipelines(pipes);
        setContactMap(new Map(cons.map((c) => [c.id, c])));
        setCompanyMap(new Map(comps.map((c) => [c.id, c])));

        // Auto-select default or first pipeline
        const def = pipes.find((p) => p.is_default) ?? pipes[0];
        if (def) setSelectedPipe(def.id);
      })
      .catch(() => setPageErr("Failed to load pipelines."));
  }, [orgId]);

  // Fetch stages + deals when pipeline changes
  const fetchBoard = useCallback(
    async (pipelineId: string) => {
      if (!pipelineId) return;
      setLoading(true);
      try {
        const [s, d] = await Promise.all([
          listStages(orgId, pipelineId),
          listDeals(orgId).then((r) =>
            r.deals.filter((x) => x.pipeline_id === pipelineId),
          ),
        ]);
        setStages(s);
        setDeals(d);
      } catch {
        setPageErr("Failed to load board.");
      } finally {
        setLoading(false);
      }
    },
    [orgId],
  );

  useEffect(() => {
    if (selectedPipe) fetchBoard(selectedPipe);
  }, [selectedPipe, fetchBoard]);

  // Group deals by stage
  const dealsByStage = useMemo(() => {
    const map = new Map<string, Deal[]>();
    stages.forEach((s) => map.set(s.id, []));
    deals.forEach((d) => {
      const arr = map.get(d.stage_id) ?? [];
      map.set(d.stage_id, [...arr, d]);
    });
    return map;
  }, [stages, deals]);

  // DnD handlers
  const handleDragStart = (event: DragStartEvent) => {
    const deal = deals.find((d) => d.id === event.active.id);
    setActiveDeal(deal ?? null);
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    setActiveDeal(null);
    const { active, over } = event;
    if (!over) return;

    const dealId = active.id as string;
    const newStageId = over.id as string;
    const oldStageId = active.data.current?.fromStageId as string;

    if (newStageId === oldStageId || !canMoveStage) return;

    // Optimistic update
    setDeals((prev) =>
      prev.map((d) => (d.id === dealId ? { ...d, stage_id: newStageId } : d)),
    );

    try {
      const updated = await moveDeal(orgId, dealId, newStageId);
      setDeals((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
    } catch {
      setPageErr("Failed to move deal.");
      // Revert optimistic update
      setDeals((prev) =>
        prev.map((d) => (d.id === dealId ? { ...d, stage_id: oldStageId } : d)),
      );
    }
  };

  // Open create deal drawer
  const openCreate = (stage?: Stage) => {
    openDrawer({
      title: "New deal",
      width: "md",
      content: (
        <DealForm
          orgId={orgId}
          defaultPipelineId={selectedPipe}
          defaultStageId={stage?.id}
          onSave={async (values) => {
            const created = await createDeal(orgId, {
              title: values.title,
              value: values.value,
              currency: values.currency,
              pipeline_id: values.pipeline_id,
              stage_id: values.stage_id,
              contact_id: values.contact_id || undefined,
              company_id: values.company_id || undefined,
              close_date: values.close_date || undefined,
            });
            setDeals((prev) => [...prev, created]);
          }}
        />
      ),
    });
  };

  // Open edit deal drawer
  const openEdit = (deal: Deal) => {
    openDrawer({
      title: "Edit deal",
      width: "md",
      content: (
        <DealForm
          deal={deal}
          orgId={orgId}
          onSave={async (values) => {
            const updated = await updateDeal(orgId, deal.id, {
              title: values.title || undefined,
              value: values.value,
              currency: values.currency || undefined,
              contact_id: values.contact_id || undefined,
              company_id: values.company_id || undefined,
              close_date: values.close_date || undefined,
            });
            setDeals((prev) =>
              prev.map((d) => (d.id === updated.id ? updated : d)),
            );
          }}
        />
      ),
    });
  };

  const handleWon = async (deal: Deal) => {
    try {
      const updated = await markDealWon(orgId, deal.id);
      setDeals((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
    } catch {
      setPageErr("Failed to mark deal as won.");
    }
  };

  const handleLost = async (deal: Deal) => {
    try {
      const updated = await markDealLost(orgId, deal.id);
      setDeals((prev) => prev.map((d) => (d.id === updated.id ? updated : d)));
    } catch {
      setPageErr("Failed to mark deal as lost.");
    }
  };

  const totalPipelineValue = deals
    .filter((d) => d.status === "open")
    .reduce((s, d) => s + d.value, 0);

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* Header */}
      <div className="flex items-center justify-between px-6 md:px-8 pt-6 pb-4 flex-shrink-0">
        <div className="flex items-center gap-4">
          <div>
            <h1
              className="text-2xl font-bold text-[var(--text-primary)]"
              style={{
                fontFamily: "var(--font-syne, Syne, sans-serif)",
                letterSpacing: "-0.02em",
              }}
            >
              Pipeline
            </h1>
            {!loading && (
              <p className="text-sm text-[var(--text-muted)]">
                {deals.filter((d) => d.status === "open").length} open deals
                {totalPipelineValue > 0 &&
                  ` · ${formatCurrency(totalPipelineValue)}`}
              </p>
            )}
          </div>

          {/* Pipeline selector */}
          {pipelines.length > 0 && (
            <select
              value={selectedPipe}
              onChange={(e) => setSelectedPipe(e.target.value)}
              className="
                px-3 py-1.5 rounded-lg text-sm
                bg-[var(--bg-elevated)] border border-[var(--border)]
                text-[var(--text-secondary)] outline-none
                focus:border-purple-500 transition-colors
              "
            >
              {pipelines.map((p) => (
                <option
                  key={p.id}
                  value={p.id}
                  style={{ background: "var(--bg-elevated)" }}
                >
                  {p.name}
                </option>
              ))}
            </select>
          )}
        </div>

        {canCreate && (
          <button
            onClick={() => openCreate()}
            className="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors"
          >
            <Plus size={15} />
            New deal
          </button>
        )}
      </div>

      {pageErr && (
        <div className="mx-6 md:mx-8 mb-3 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20 flex-shrink-0">
          {pageErr}
        </div>
      )}

      {/* Board */}
      {loading ? (
        <div className="flex items-center gap-3 px-8 py-16 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading board…
        </div>
      ) : stages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <p className="text-sm text-[var(--text-muted)] mb-1">
            No stages in this pipeline
          </p>
          <p className="text-xs text-[var(--text-muted)]">
            Add stages to your pipeline to start tracking deals
          </p>
        </div>
      ) : (
        <div className="flex-1 overflow-x-auto overflow-y-hidden px-6 md:px-8 pb-6">
          <DndContext
            sensors={sensors}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
          >
            <div className="flex gap-4 h-full min-h-[500px]">
              {stages
                .slice()
                .sort((a, b) => a.position - b.position)
                .map((stage) => (
                  <KanbanColumn
                    key={stage.id}
                    stage={stage}
                    deals={dealsByStage.get(stage.id) ?? []}
                    contactMap={contactMap}
                    companyMap={companyMap}
                    onNewDeal={openCreate}
                    onEdit={openEdit}
                    onWon={handleWon}
                    onLost={handleLost}
                    canCreate={canCreate}
                  />
                ))}
            </div>

            {/* Drag overlay — renders on top while dragging */}
            <DragOverlay>
              {activeDeal && (
                <DealCard
                  deal={activeDeal}
                  contactMap={contactMap}
                  companyMap={companyMap}
                  onEdit={() => {}}
                  onWon={() => {}}
                  onLost={() => {}}
                  isDragOverlay
                />
              )}
            </DragOverlay>
          </DndContext>
        </div>
      )}
    </div>
  );
}
