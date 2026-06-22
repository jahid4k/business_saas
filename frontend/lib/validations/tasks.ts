import { z } from "zod";

export const createTaskSchema = z.object({
  title: z
    .string()
    .min(1, "Title is required")
    .max(255, "Title must be under 255 characters")
    .trim(),
  description: z
    .string()
    .max(2000, "Description must be under 2000 characters")
    .trim()
    .optional()
    .or(z.literal("")),
  status: z
    .enum(["todo", "in_progress", "done"], {
      errorMap: () => ({ message: "Status must be todo, in_progress, or done" }),
    })
    .default("todo"),
});

export type CreateTaskInput = z.infer<typeof createTaskSchema>;

export const updateTaskSchema = z.object({
  title: z
    .string()
    .min(1, "Title is required")
    .max(255, "Title must be under 255 characters")
    .trim()
    .optional(),
  description: z
    .string()
    .max(2000, "Description must be under 2000 characters")
    .trim()
    .optional()
    .nullable(),
  status: z
    .enum(["todo", "in_progress", "done"])
    .optional(),
});

export type UpdateTaskInput = z.infer<typeof updateTaskSchema>;
