import clsx from "clsx";

interface StatusDotProps {
  status: "ok" | "error" | "loading";
  label?: string;
}

export function StatusDot({ status, label }: StatusDotProps) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        className={clsx(
          "relative inline-flex h-2 w-2 rounded-full shrink-0",
          status === "ok" && "bg-success",
          status === "error" && "bg-error",
          status === "loading" && "bg-brand-500 animate-pulse",
        )}
      >
        {status === "ok" && (
          <span className="absolute inline-flex h-full w-full rounded-full bg-success opacity-50 animate-ping" />
        )}
      </span>
      {label && (
        <span
          className={clsx(
            "text-xs font-medium",
            status === "ok" && "text-success",
            status === "error" && "text-error",
            status === "loading" && "text-gray-500",
          )}
        >
          {label}
        </span>
      )}
    </span>
  );
}
