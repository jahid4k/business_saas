import clsx from "clsx";

type BadgeVariant =
  | "success"
  | "warning"
  | "error"
  | "info"
  | "neutral"
  | "blue";

interface BadgeProps {
  variant?: BadgeVariant;
  children: React.ReactNode;
  className?: string;
}

export function Badge({
  variant = "neutral",
  children,
  className,
}: BadgeProps) {
  return (
    <span
      className={clsx(
        "inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium",
        variant === "success" && "bg-success-light text-success",
        variant === "warning" && "bg-warning-light text-warning",
        variant === "error" && "bg-error-light   text-error",
        variant === "info" && "bg-info-light     text-info",
        variant === "blue" && "bg-brand-50 text-brand-700",
        variant === "neutral" && "bg-gray-100 text-gray-600",
        className,
      )}
    >
      {children}
    </span>
  );
}
