import { z } from "zod";
import { nameToSlug } from "@/lib/utils";

export const createOrgSchema = z.object({
  name: z
    .string()
    .min(2, "Organization name must be at least 2 characters")
    .max(120, "Organization name must be under 120 characters")
    .trim(),
  slug: z
    .string()
    .min(2, "Slug must be at least 2 characters")
    .max(60, "Slug must be under 60 characters")
    .regex(
      /^[a-z0-9-]+$/,
      "Slug can only contain lowercase letters, numbers, and hyphens",
    )
    .trim(),
});

export type CreateOrgInput = z.infer<typeof createOrgSchema>;

/** Auto-derive a slug from a name as the user types */
export function deriveSlug(name: string): string {
  return nameToSlug(name);
}
