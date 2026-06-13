import { ButtonHTMLAttributes, forwardRef } from "react";
import clsx from "clsx";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  isLoading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = "primary",
      size = "md",
      isLoading,
      className,
      children,
      disabled,
      ...props
    },
    ref,
  ) => (
    <button
      ref={ref}
      disabled={disabled || isLoading}
      className={clsx(
        "inline-flex items-center justify-center gap-2 font-medium transition-all",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-1",
        "disabled:opacity-50 disabled:cursor-not-allowed select-none",
        size === "sm" && "px-3 py-1.5 text-xs rounded",
        size === "md" && "px-4 py-2 text-sm rounded",
        size === "lg" && "px-5 py-2.5 text-sm rounded-md",
        variant === "primary" &&
          "bg-brand-600 text-white hover:bg-brand-700 active:bg-brand-800 shadow-xs",
        variant === "secondary" &&
          "bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 hover:border-gray-400 shadow-xs",
        variant === "ghost" &&
          "text-gray-600 hover:text-gray-900 hover:bg-gray-100",
        variant === "danger" &&
          "bg-error-light text-error border border-error/20 hover:bg-red-100",
        className,
      )}
      {...props}
    >
      {isLoading ? <Spinner /> : null}
      {children}
    </button>
  ),
);
Button.displayName = "Button";

function Spinner() {
  return (
    <svg className="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24">
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  );
}
