"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import {
  createTaskSchema,
  updateTaskSchema,
  type CreateTaskInput,
} from "@/lib/validations/tasks";
import { apiPost, apiPatch } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { useOrg } from "@/hooks/useOrg";
import { ApiError } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Task } from "@/types/domain";

interface TaskFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  task: Task | null; // null = create mode, non-null = edit mode
}

export function TaskForm({ open, onOpenChange, task }: TaskFormProps) {
  const queryClient = useQueryClient();
  const { orgId } = useOrg();
  const isEditing = Boolean(task);

  const form = useForm<CreateTaskInput>({
    resolver: zodResolver(isEditing ? updateTaskSchema : createTaskSchema),
    defaultValues: {
      title: "",
      description: "",
      status: "todo",
    },
  });

  // Populate form when editing
  useEffect(() => {
    if (task) {
      form.reset({
        title: task.title,
        description: task.description ?? "",
        status: task.status,
      });
    } else {
      form.reset({ title: "", description: "", status: "todo" });
    }
  }, [task, form]);

  const mutation = useMutation({
    mutationFn: (data: CreateTaskInput) => {
      if (isEditing && task) {
        return apiPatch<{ task: Task }>(
          `api/v1/organizations/${orgId}/tasks/${task.id}`,
          data,
        );
      }
      return apiPost<{ task: Task }>(
        `api/v1/organizations/${orgId}/tasks`,
        data,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.tasks.list(orgId ?? ""),
      });
      toast.success(isEditing ? "Task updated" : "Task created");
      onOpenChange(false);
    },
    onError: (error) => {
      if (error instanceof ApiError && error.hasFieldErrors) {
        Object.entries(error.fields!).forEach(([field, message]) => {
          form.setError(field as keyof CreateTaskInput, { message });
        });
      }
      // Global mutation error handler in query-client.ts shows toast for other errors
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEditing ? "Edit task" : "Create task"}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? "Update the task details below"
              : "Add a new task to your workspace"}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((d) => mutation.mutate(d))}
            className="space-y-4"
          >
            <FormField
              control={form.control}
              name="title"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Title</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Task title"
                      autoFocus
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    Description{" "}
                    <span className="text-xs font-normal text-muted-foreground">
                      (optional)
                    </span>
                  </FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder="Add more details about this task..."
                      rows={3}
                      {...field}
                      value={field.value ?? ""}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="status"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Status</FormLabel>
                  <Select
                    onValueChange={field.onChange}
                    defaultValue={field.value}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select status" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="todo">To do</SelectItem>
                      <SelectItem value="in_progress">In progress</SelectItem>
                      <SelectItem value="done">Done</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                Cancel
              </Button>
              <Button type="submit" loading={mutation.isPending}>
                {isEditing ? "Save changes" : "Create task"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
