// src/app/(dashboard)/[orgId]/crm/pipeline/page.tsx
"use client";

import { use, useCallback, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  DragEndEvent,
  DragOverlay,
  DragStartEvent,
  PointerSensor,
  useSensor,
  useSensors,
  useDroppable,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  Plus,
  Trophy,
  X,
  Pencil,
  Building2,
  User,
  Calendar,
  Loader2,
  ChevronDown,
} from "lucide-react";
import { usePermissionStore } from "@/stores/permissionStore";
import { useDrawer } from "@/contexts/DrawerContext";
import { useIsMobile } from "@/hooks/useIsMobile";
import { toast } from "sonner";
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
import { queryKeys } from "@/lib/queryKeys";
import DealForm from "@/components/crm/deals/DealForm";
import type {
  Deal,
  Stage,
  Contact,
  Company,
  CreateDealPayload,
  UpdateDealPayload,
} from "@/types/crm";

// ── Helpers ───────────────────────────────────────────────────────────────────
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

// ── Mobile deal card ──────────────────────────────────────────────────────────
// Used only below the lg breakpoint. Has an explicit "Move to →" dropdown
// instead of drag-and-drop (which conflicts with touch scrolling gestures).
interface MobileDealCardProps {
  deal: Deal;
  stages: Stage[];
  contactMap: Map<string, Contact>;
  companyMap: Map<string, Company>;
  onEdit: (deal: Deal) => void;
  onMove: (dealId: string, stageId: string) => void;
  onWon: (deal: Deal) => void;
  onLost: (deal: Deal) => void;
}

function MobileDealCard({
  deal,
  stages,
  contactMap,
  companyMap,
  onEdit,
  onMove,
  onWon,
  onLost,
}: MobileDealCardProps) {
  const [moveOpen, setMoveOpen] = useState(false);
  const contact = contactMap.get(deal.contact_id ?? "");
  const company = companyMap.get(deal.company_id ?? "");
  const otherStages = stages.filter((s) => s.id !== deal.stage_id);

  return (
    <div className="rounded-2xl p-4 border bg-[var(--bg-surface)] border-[var(--border)] active:scale-[0.99] transition-transform">
      {/* Title row */}
      <div className="flex items-start justify-between gap-2 mb-1">
        <p
          className="text-sm font-semibold text-[var(--text-primary)] leading-snug"
          style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
        >
          {deal.title}
        </p>
        <button
          onClick={() => onEdit(deal)}
          className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] flex-shrink-0 transition-colors"
        >
          <Pencil size={12} />
        </button>
      </div>

      {/* Value */}
      <p className="text-xl font-bold text-[var(--text-primary)] mb-3">
        {formatCurrency(deal.value, deal.currency)}
      </p>

      {/* Meta */}
      <div className="space-y-1.5 mb-4">
        {company && (
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <Building2 size={11} className="flex-shrink-0" />
            <span className="truncate">{company.name}</span>
          </div>
        )}
        {contact && (
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <User size={11} className="flex-shrink-0" />
            <span className="truncate">
              {contact.first_name} {contact.last_name ?? ""}
            </span>
          </div>
        )}
        {deal.close_date && (
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <Calendar size={11} className="flex-shrink-0" />
            <span>{formatDate(deal.close_date)}</span>
          </div>
        )}
      </div>

      {/* Actions */}
      {deal.status === "open" ? (
        <div className="flex items-center gap-2 pt-3 border-t border-[var(--border)]">
          <button
            onClick={() => onWon(deal)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 active:bg-emerald-500/20 transition-colors"
          >
            <Trophy size={11} />
            Won
          </button>
          <button
            onClick={() => onLost(deal)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold text-red-400 bg-red-500/10 border border-red-500/20 active:bg-red-500/20 transition-colors"
          >
            <X size={11} />
            Lost
          </button>

          {/* Move to stage */}
          {otherStages.length > 0 && (
            <div className="relative ml-auto">
              <button
                onClick={() => setMoveOpen((v) => !v)}
                className="flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold text-[var(--text-secondary)] bg-[var(--bg-elevated)] border border-[var(--border)] active:bg-[var(--bg-base)] transition-colors"
              >
                Move
                <ChevronDown
                  size={11}
                  style={{
                    transform: moveOpen ? "rotate(180deg)" : "rotate(0deg)",
                    transition: "transform 150ms ease",
                  }}
                />
              </button>

              {moveOpen && (
                <>
                  {/* Tap-away backdrop */}
                  <div
                    className="fixed inset-0 z-10"
                    onClick={() => setMoveOpen(false)}
                    aria-hidden="true"
                  />
                  {/* Stage list — appears above button */}
                  <div
                    className="absolute right-0 bottom-full mb-2 w-52 rounded-xl overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border)] shadow-2xl z-20"
                    style={{ maxHeight: "60vh", overflowY: "auto" }}
                  >
                    <p className="px-3 py-2 text-[0.65rem] font-semibold text-[var(--text-muted)] uppercase tracking-wider border-b border-[var(--border)]">
                      Move to stage
                    </p>
                    {otherStages.map((stage) => (
                      <button
                        key={stage.id}
                        onClick={() => {
                          onMove(deal.id, stage.id);
                          setMoveOpen(false);
                        }}
                        className="w-full flex items-center gap-2 px-3 py-3 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)] transition-colors text-left"
                        style={{
                          fontFamily: "var(--font-inter, Inter, sans-serif)",
                        }}
                      >
                        <span className="w-1.5 h-1.5 rounded-full bg-purple-500 flex-shrink-0" />
                        {stage.name}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      ) : (
        <div className="pt-3 border-t border-[var(--border)]">
          <span
            className={`text-xs font-semibold ${STATUS_COLOR[deal.status]}`}
          >
            {deal.status === "won" ? "✓ Won" : "✗ Lost"}
          </span>
        </div>
      )}
    </div>
  );
}

// ── Desktop deal card ─────────────────────────────────────────────────────────
// useSortable (from @dnd-kit/sortable) replaces useDraggable.
// It provides the same drag handles but also tells the SortableContext
// how to animate sibling cards out of the way during a within-column drag.
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
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: deal.id,
    // fromStageId is read in handleDragEnd to distinguish
    // within-column reorder from cross-column move.
    data: { fromStageId: deal.stage_id },
  });

  const contact = contactMap.get(deal.contact_id ?? "");
  const company = companyMap.get(deal.company_id ?? "");

  const style: React.CSSProperties = {
    // CSS.Transform.toString handles scaleX/scaleY from the sortable context
    // correctly (unlike the raw translate3d we used with useDraggable).
    transform: CSS.Transform.toString(transform),
    transition,
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
        transition-colors duration-150
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

      {/* Meta */}
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

      {/* Won / Lost buttons */}
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

// ── Desktop kanban column ─────────────────────────────────────────────────────
// SortableContext wraps the cards so dnd-kit can animate siblings during
// within-column drags. useDroppable still handles cross-column drops.
interface KanbanColumnProps {
  stage: Stage;
  orderedDeals: Deal[];
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
  orderedDeals,
  contactMap,
  companyMap,
  onNewDeal,
  onEdit,
  onWon,
  onLost,
  canCreate,
}: KanbanColumnProps) {
  const { setNodeRef, isOver } = useDroppable({ id: stage.id });

  const totalValue = orderedDeals
    .filter((d) => d.status === "open")
    .reduce((s, d) => s + d.value, 0);

  const sortedIds = orderedDeals.map((d) => d.id);

  return (
    <div
      className="flex flex-col flex-shrink-0 h-full min-h-0"
      style={{ width: 272 }}
    >
      {/* Header — flex-shrink-0 so only the card list scrolls */}
      <div className="flex items-center justify-between px-1 mb-3 flex-shrink-0">
        <div>
          <p
            className="text-sm font-semibold text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
          >
            {stage.name} <span className="text-xs text-[var(--text-muted)] font-normal ml-1">({stage.probability}%)</span>
          </p>
          <p className="text-xs text-[var(--text-muted)]">
            {orderedDeals.length} deal{orderedDeals.length !== 1 ? "s" : ""}
            {totalValue > 0 && ` · ${formatCurrency(totalValue)}`}
            {totalValue > 0 && stage.probability > 0 && stage.probability < 100 && (
              <span className="text-purple-400">
                {` · Expected: ${formatCurrency(totalValue * (stage.probability / 100))}`}
              </span>
            )}
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

      {/* Drop zone — independently scrollable */}
      <div
        ref={setNodeRef}
        className={`
          flex-1 min-h-0 overflow-y-auto rounded-xl p-2 space-y-2
          transition-colors duration-150
          ${
            isOver
              ? "bg-purple-500/8 border border-purple-500/30"
              : "bg-[var(--bg-elevated)]/50 border border-[var(--border)]/60"
          }
        `}
      >
        <SortableContext
          items={sortedIds}
          strategy={verticalListSortingStrategy}
        >
          {orderedDeals.map((deal) => (
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
        </SortableContext>

        {orderedDeals.length === 0 && (
          <div className="flex items-center justify-center h-24">
            <p className="text-xs text-[var(--text-muted)]">Drop deals here</p>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────
export default function PipelinePage({
  params,
}: {
  params: Promise<{ orgId: string }>;
}) {
  const { orgId } = use(params);
  const { hasPermission } = usePermissionStore();
  const { openDrawer } = useDrawer();
  const queryClient = useQueryClient();
  const isMobile = useIsMobile();

  // ── UI state ─────────────────────────────────────────────────────────────
  const [selectedPipe, setSelectedPipe] = useState<string>("");
  const [mobileStageId, setMobileStageId] = useState<string>("");
  const [activeDeal, setActiveDeal] = useState<Deal | null>(null);
  const [cardOrder, setCardOrder] = useState<Map<string, string[]>>(new Map());
  // Within-column card order — client-side only.
  // Persists for this browser session; resets on page refresh.
  // Full persistence needs a `sort_order` column on crm_deals + a backend
  // reorder endpoint, which can be added as a future backend step.

  const canCreate = hasPermission("crm.deals.create");
  const canMoveStage = hasPermission("crm.deals.move_stage");

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
  );

  // ── Queries ───────────────────────────────────────────────────────────────
  const pipelinesQuery = useQuery({
    queryKey: queryKeys.crm.pipelines.list(orgId),
    queryFn: () => listPipelines(orgId),
  });

  const contactsQuery = useQuery({
    queryKey: queryKeys.crm.contacts.list(orgId),
    queryFn: () => listContacts(orgId).then((r) => r.contacts),
  });

  const companiesQuery = useQuery({
    queryKey: queryKeys.crm.companies.list(orgId),
    queryFn: () => listCompanies(orgId).then((r) => r.companies),
  });

  const pipelines = pipelinesQuery.data ?? [];
  const defaultPipelineId =
    pipelines.find((p) => p.is_default)?.id ?? pipelines[0]?.id ?? "";
  const activePipelineId = selectedPipe || defaultPipelineId;

  const stagesQuery = useQuery({
    queryKey: queryKeys.crm.pipelines.stages(orgId, activePipelineId),
    queryFn: () => listStages(orgId, activePipelineId),
    enabled: !!activePipelineId,
  });

  const dealsKey = queryKeys.crm.deals.list(orgId);
  const dealsQuery = useQuery({
    queryKey: dealsKey,
    queryFn: () => listDeals(orgId).then((r) => r.deals),
    enabled: !!activePipelineId,
  });

  // ── Derived data ──────────────────────────────────────────────────────────
  const contactMap = useMemo(
    () => new Map((contactsQuery.data ?? []).map((c) => [c.id, c])),
    [contactsQuery.data],
  );

  const companyMap = useMemo(
    () => new Map((companiesQuery.data ?? []).map((c) => [c.id, c])),
    [companiesQuery.data],
  );

  const sortedStages = useMemo(
    () => [...(stagesQuery.data ?? [])].sort((a, b) => a.position - b.position),
    [stagesQuery.data],
  );

  // Set of stage IDs — used in handleDragEnd to tell apart a stage-column
  // drop target from a deal-card drop target.
  const stageIdSet = useMemo(
    () => new Set(sortedStages.map((s) => s.id)),
    [sortedStages],
  );

  // All deals in the active pipeline, keyed by stage.
  const pipelineDeals = useMemo(
    () =>
      (dealsQuery.data ?? []).filter((d) => d.pipeline_id === activePipelineId),
    [dealsQuery.data, activePipelineId],
  );

  const dealsByStage = useMemo(() => {
    const map = new Map<string, Deal[]>();
    sortedStages.forEach((s) => map.set(s.id, []));
    pipelineDeals.forEach((d) => {
      const arr = map.get(d.stage_id) ?? [];
      map.set(d.stage_id, [...arr, d]);
    });
    return map;
  }, [sortedStages, pipelineDeals]);

  // ── Card order helpers ────────────────────────────────────────────────────
  // Returns ordered deal IDs for a stage, merging any client-side sort
  // preference with the server list (handles adds/removes).
  const getOrderedDealIds = useCallback(
    (stageId: string, order: Map<string, string[]>): string[] => {
      const all = dealsByStage.get(stageId) ?? [];
      const stored = order.get(stageId);
      if (!stored) return all.map((d) => d.id);

      const allSet = new Set(all.map((d) => d.id));
      const valid = stored.filter((id) => allSet.has(id));
      const validSet = new Set(valid);
      const fresh = all.filter((d) => !validSet.has(d.id)).map((d) => d.id);
      return [...valid, ...fresh];
    },
    [dealsByStage],
  );

  const getOrderedDeals = useCallback(
    (stageId: string): Deal[] => {
      const ids = getOrderedDealIds(stageId, cardOrder);
      const dealMap = new Map(
        (dealsByStage.get(stageId) ?? []).map((d) => [d.id, d]),
      );
      return ids
        .map((id) => dealMap.get(id))
        .filter((d): d is Deal => d !== undefined);
    },
    [getOrderedDealIds, cardOrder, dealsByStage],
  );

  // Active mobile stage
  const activeMobileStageId = mobileStageId || (sortedStages[0]?.id ?? "");

  // ── Mutations ─────────────────────────────────────────────────────────────
  const moveDealMutation = useMutation({
    mutationFn: ({ dealId, stageId }: { dealId: string; stageId: string }) =>
      moveDeal(orgId, dealId, stageId),

    onMutate: async ({ dealId, stageId: newStageId }) => {
      await queryClient.cancelQueries({ queryKey: dealsKey });
      const previousDeals = queryClient.getQueryData<Deal[]>(dealsKey);
      const movingDeal = (previousDeals ?? []).find((d) => d.id === dealId);

      // Optimistically update the server-synced deal list
      queryClient.setQueryData<Deal[]>(dealsKey, (old) =>
        (old ?? []).map((d) =>
          d.id === dealId ? { ...d, stage_id: newStageId } : d,
        ),
      );

      // Sync the client-side card order for both affected stages
      if (movingDeal) {
        setCardOrder((prev) => {
          const next = new Map(prev);
          const srcIds = getOrderedDealIds(movingDeal.stage_id, prev);
          next.set(
            movingDeal.stage_id,
            srcIds.filter((id) => id !== dealId),
          );
          const dstIds = getOrderedDealIds(newStageId, prev);
          if (!dstIds.includes(dealId)) {
            next.set(newStageId, [...dstIds, dealId]);
          }
          return next;
        });
      }

      return { previousDeals };
    },

    onError: (_err, { dealId, stageId: newStageId }, context) => {
      if (context?.previousDeals) {
        queryClient.setQueryData(dealsKey, context.previousDeals);
      }
      // Clear the order for both affected stages so they revert to server order
      const srcDeal = (context?.previousDeals ?? []).find(
        (d) => d.id === dealId,
      );
      if (srcDeal) {
        setCardOrder((prev) => {
          const next = new Map(prev);
          next.delete(srcDeal.stage_id);
          next.delete(newStageId);
          return next;
        });
      }
      toast.error("Failed to move deal.");
    },

    onSuccess: (updatedDeal) => {
      queryClient.setQueryData<Deal[]>(dealsKey, (old) =>
        (old ?? []).map((d) => (d.id === updatedDeal.id ? updatedDeal : d)),
      );
    },
  });

  const createDealMutation = useMutation({
    mutationFn: (payload: CreateDealPayload) => createDeal(orgId, payload),
    onSuccess: (created) => {
      queryClient.setQueryData<Deal[]>(dealsKey, (old) => [
        ...(old ?? []),
        created,
      ]);
      toast.success("Deal created.");
    },
  });

  const updateDealMutation = useMutation({
    mutationFn: ({
      dealId,
      payload,
    }: {
      dealId: string;
      payload: UpdateDealPayload;
    }) => updateDeal(orgId, dealId, payload),
    onSuccess: (updated) => {
      queryClient.setQueryData<Deal[]>(dealsKey, (old) =>
        (old ?? []).map((d) => (d.id === updated.id ? updated : d)),
      );
      toast.success("Deal updated.");
    },
  });

  const wonMutation = useMutation({
    mutationFn: (dealId: string) => markDealWon(orgId, dealId),
    onSuccess: (updated) => {
      queryClient.setQueryData<Deal[]>(dealsKey, (old) =>
        (old ?? []).map((d) => (d.id === updated.id ? updated : d)),
      );
      toast.success("Deal won! 🎉");
    },
    onError: () => toast.error("Failed to mark deal as won."),
  });

  const lostMutation = useMutation({
    mutationFn: (dealId: string) => markDealLost(orgId, dealId),
    onSuccess: (updated) => {
      queryClient.setQueryData<Deal[]>(dealsKey, (old) =>
        (old ?? []).map((d) => (d.id === updated.id ? updated : d)),
      );
      toast.success("Deal marked as lost.");
    },
    onError: () => toast.error("Failed to mark deal as lost."),
  });

  // ── DnD handlers (desktop only) ───────────────────────────────────────────
  const handleDragStart = (event: DragStartEvent) => {
    const deal = pipelineDeals.find((d) => d.id === event.active.id);
    setActiveDeal(deal ?? null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveDeal(null);
    const { active, over } = event;
    if (!over || !active) return;

    const draggedId = active.id as string;
    const fromStageId = (active.data.current as { fromStageId?: string })
      ?.fromStageId;

    if (!fromStageId) return;

    // ── Case 1: dropped on a stage column (the useDroppable zone) ──────────
    if (stageIdSet.has(over.id as string)) {
      const toStageId = over.id as string;
      if (toStageId !== fromStageId && canMoveStage) {
        moveDealMutation.mutate({ dealId: draggedId, stageId: toStageId });
      }
      return;
    }

    // ── Case 2: dropped on another deal card ───────────────────────────────
    const toStageId = (over.data.current as { fromStageId?: string })
      ?.fromStageId;

    if (!toStageId) return;

    if (toStageId !== fromStageId) {
      // Cross-column: card dropped on top of a card in a different stage
      if (canMoveStage) {
        moveDealMutation.mutate({ dealId: draggedId, stageId: toStageId });
      }
    } else {
      // Within-column: reorder using arrayMove
      setCardOrder((prev) => {
        const ids = getOrderedDealIds(fromStageId, prev);
        const oldIndex = ids.indexOf(draggedId);
        const newIndex = ids.indexOf(over.id as string);
        if (oldIndex === -1 || newIndex === -1 || oldIndex === newIndex) {
          return prev;
        }
        return new Map(prev).set(
          fromStageId,
          arrayMove(ids, oldIndex, newIndex),
        );
      });
    }
  };

  // ── Drawer openers ────────────────────────────────────────────────────────
  const openCreate = (stage?: Stage) => {
    openDrawer({
      title: "New deal",
      width: "md",
      content: (
        <DealForm
          orgId={orgId}
          defaultPipelineId={activePipelineId}
          defaultStageId={stage?.id}
          onSave={async (values) => {
            await createDealMutation.mutateAsync({
              title: values.title,
              value: values.value,
              currency: values.currency,
              pipeline_id: values.pipeline_id,
              stage_id: values.stage_id,
              contact_id: values.contact_id || undefined,
              company_id: values.company_id || undefined,
              close_date: values.close_date || undefined,
            });
          }}
        />
      ),
    });
  };

  const openEdit = (deal: Deal) => {
    openDrawer({
      title: "Edit deal",
      width: "md",
      content: (
        <DealForm
          deal={deal}
          orgId={orgId}
          onSave={async (values) => {
            await updateDealMutation.mutateAsync({
              dealId: deal.id,
              payload: {
                title: values.title || undefined,
                value: values.value,
                currency: values.currency || undefined,
                contact_id: values.contact_id || undefined,
                company_id: values.company_id || undefined,
                close_date: values.close_date || undefined,
              },
            });
          }}
        />
      ),
    });
  };

  const handleWon = (deal: Deal) => wonMutation.mutate(deal.id);
  const handleLost = (deal: Deal) => lostMutation.mutate(deal.id);
  const handleMove = (dealId: string, stageId: string) =>
    moveDealMutation.mutate({ dealId, stageId });

  // ── Shared loading / error states ─────────────────────────────────────────
  const isBoardLoading =
    pipelinesQuery.isPending ||
    (!!activePipelineId && (stagesQuery.isPending || dealsQuery.isPending));

  const bannerError = pipelinesQuery.isError
    ? "Failed to load pipelines."
    : stagesQuery.isError || dealsQuery.isError
      ? "Failed to load board."
      : null;

  const totalOpenValue = pipelineDeals
    .filter((d) => d.status === "open")
    .reduce((s, d) => s + d.value, 0);

  const openDealCount = pipelineDeals.filter((d) => d.status === "open").length;

  // ── Header (shared) ───────────────────────────────────────────────────────
  const header = (
    <div className="flex items-center justify-between px-4 md:px-8 pt-5 pb-4 flex-shrink-0 gap-3">
      <div className="flex items-center gap-3 min-w-0">
        <div className="min-w-0">
          <h1
            className="text-xl md:text-2xl font-bold text-[var(--text-primary)] truncate"
            style={{
              fontFamily: "var(--font-syne, Syne, sans-serif)",
              letterSpacing: "-0.02em",
            }}
          >
            Pipeline
          </h1>
          {!isBoardLoading && (
            <p className="text-xs md:text-sm text-[var(--text-muted)] truncate">
              {openDealCount} open
              {totalOpenValue > 0 && ` · ${formatCurrency(totalOpenValue)}`}
            </p>
          )}
        </div>

        {pipelines.length > 0 && (
          <select
            value={activePipelineId}
            onChange={(e) => {
              setSelectedPipe(e.target.value);
              setMobileStageId(""); // reset mobile tab on pipeline change
            }}
            className="hidden sm:block px-3 py-1.5 rounded-lg text-sm bg-[var(--bg-elevated)] border border-[var(--border)] text-[var(--text-secondary)] outline-none focus:border-purple-500 transition-colors flex-shrink-0"
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
          className="flex items-center gap-1.5 px-3 md:px-4 py-2 md:py-2.5 rounded-lg text-xs md:text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 transition-colors flex-shrink-0"
        >
          <Plus size={14} />
          <span className="hidden sm:inline">New deal</span>
          <span className="sm:hidden">New</span>
        </button>
      )}
    </div>
  );

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="flex flex-col h-full min-h-0">
      {header}

      {bannerError && (
        <div className="mx-4 md:mx-8 mb-3 px-4 py-3 rounded-lg text-sm text-red-400 bg-red-500/8 border border-red-500/20 flex-shrink-0">
          {bannerError}
        </div>
      )}

      {isBoardLoading ? (
        <div className="flex items-center gap-3 px-8 py-16 text-sm text-[var(--text-muted)]">
          <Loader2 size={15} className="animate-spin text-purple-500" />
          Loading board…
        </div>
      ) : sortedStages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-center px-8">
          <p className="text-sm text-[var(--text-muted)] mb-1">
            No stages in this pipeline
          </p>
          <p className="text-xs text-[var(--text-muted)]">
            Add stages to your pipeline to start tracking deals
          </p>
        </div>
      ) : isMobile ? (
        // ── Mobile: single-stage view ────────────────────────────────────────
        <div className="flex flex-col flex-1 min-h-0">
          {/* Stage tab strip */}
          <div
            className="flex gap-2 px-4 py-3 border-b border-[var(--border)] flex-shrink-0 overflow-x-auto"
            style={{ scrollbarWidth: "none" }}
          >
            {sortedStages.map((stage) => {
              const count = (dealsByStage.get(stage.id) ?? []).length;
              const active = stage.id === activeMobileStageId;
              return (
                <button
                  key={stage.id}
                  onClick={() => setMobileStageId(stage.id)}
                  className={`
                    flex-shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-semibold
                    transition-colors
                    ${
                      active
                        ? "bg-purple-600 text-white"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
                    }
                  `}
                >
                  {stage.name}
                  {count > 0 && (
                    <span
                      className={`text-[0.6rem] px-1.5 py-0.5 rounded-full ${
                        active
                          ? "bg-white/20 text-white"
                          : "bg-[var(--bg-base)] text-[var(--text-muted)]"
                      }`}
                    >
                      {count}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {/* Active stage cards */}
          <div className="flex-1 min-h-0 overflow-y-auto px-4 py-4 space-y-3">
            {getOrderedDeals(activeMobileStageId).map((deal) => (
              <MobileDealCard
                key={deal.id}
                deal={deal}
                stages={sortedStages}
                contactMap={contactMap}
                companyMap={companyMap}
                onEdit={openEdit}
                onMove={handleMove}
                onWon={handleWon}
                onLost={handleLost}
              />
            ))}

            {getOrderedDeals(activeMobileStageId).length === 0 && (
              <div className="flex flex-col items-center justify-center py-16 text-center">
                <p className="text-sm text-[var(--text-muted)] mb-1">
                  No deals in this stage
                </p>
                {canCreate && (
                  <p className="text-xs text-[var(--text-muted)]">
                    Tap &ldquo;New&rdquo; to add one
                  </p>
                )}
              </div>
            )}

            {/* Add deal shortcut at the bottom */}
            {canCreate && (
              <button
                onClick={() =>
                  openCreate(
                    sortedStages.find((s) => s.id === activeMobileStageId),
                  )
                }
                className="w-full py-3.5 rounded-2xl border-2 border-dashed border-[var(--border)] text-sm text-[var(--text-muted)] hover:border-purple-500/40 hover:text-purple-400 transition-colors flex items-center justify-center gap-2"
              >
                <Plus size={14} />
                Add deal to this stage
              </button>
            )}
          </div>
        </div>
      ) : (
        // ── Desktop: multi-column kanban with DnD ────────────────────────────
        <div className="flex-1 min-h-0 overflow-x-auto overflow-y-hidden px-8 pb-6">
          <DndContext
            sensors={sensors}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
          >
            <div className="flex gap-4 h-full">
              {sortedStages.map((stage) => (
                <KanbanColumn
                  key={stage.id}
                  stage={stage}
                  orderedDeals={getOrderedDeals(stage.id)}
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
