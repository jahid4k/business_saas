import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { format, formatDistanceToNow } from "date-fns";

// ----------------------------------------------------------
// Tailwind class merger
// ----------------------------------------------------------

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// ----------------------------------------------------------
// Date formatters
// ----------------------------------------------------------

/** "Jun 22, 2025" */
export function formatDate(date: string | Date | null | undefined): string {
  if (!date) return "—";
  try {
    return format(new Date(date), "MMM d, yyyy");
  } catch {
    return "—";
  }
}

/** "Jun 22, 2025 at 3:45 PM" */
export function formatDateTime(date: string | Date | null | undefined): string {
  if (!date) return "—";
  try {
    return format(new Date(date), "MMM d, yyyy 'at' h:mm a");
  } catch {
    return "—";
  }
}

/** "2 hours ago" / "in 3 days" */
export function formatRelative(date: string | Date | null | undefined): string {
  if (!date) return "—";
  try {
    return formatDistanceToNow(new Date(date), { addSuffix: true });
  } catch {
    return "—";
  }
}

// ----------------------------------------------------------
// Currency formatter
// ----------------------------------------------------------

export function formatCurrency(
  amount: number | null | undefined,
  currency = "USD",
): string {
  if (amount == null) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(amount);
}

// ----------------------------------------------------------
// String helpers
// ----------------------------------------------------------

/** "john doe" → "John Doe" */
export function titleCase(str: string): string {
  return str
    .toLowerCase()
    .split(" ")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/** "john-doe-corp" → "John Doe Corp" */
export function slugToTitle(slug: string): string {
  return titleCase(slug.replace(/-/g, " "));
}

/** "John Doe" → "JD" */
export function getInitials(name: string): string {
  return name
    .split(" ")
    .map((part) => part.charAt(0))
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

/** Build a URL-safe org slug from a name: "My Business" → "my-business" */
export function nameToSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

// ----------------------------------------------------------
// Error helpers
// ----------------------------------------------------------

/** Extract a human-readable message from any thrown value */
export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "An unexpected error occurred";
}
