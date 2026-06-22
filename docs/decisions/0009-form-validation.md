# ADR-0009: Forms and validation — React Hook Form + Zod

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS has forms throughout the application: login, signup, creating leads, updating deals,
inviting members, configuring roles, creating tasks, and every settings page. Forms are the
primary way users interact with the system.

Requirements for the form system:
- Type-safe — form values must be fully typed; no `any`
- Performant — re-rendering on every keystroke is unacceptable in complex forms
- Validation — errors shown inline per field, matching backend validation logic where possible
- Server error integration — API errors must be mapped back to specific fields
- Accessible — errors announced to screen readers, labels properly associated
- Minimal boilerplate — adding a new field should not require touching 5 files

---

## Decision

Use **React Hook Form** for form state management and **Zod** for schema validation.
Connect them with the official `@hookform/resolvers/zod` resolver.

---

## Reasoning

### Why React Hook Form

React Hook Form uses **uncontrolled inputs** by default — it registers inputs via `ref`
and reads values on change without re-rendering the parent component. A form with 20 fields
renders once on mount, not 20 times per keystroke.

Controlled React forms (with `value` and `onChange` on every input) re-render the entire form
tree on every keystroke. For complex forms (lead creation with 15 fields), this is measurably
slow and causes input lag on lower-end devices.

React Hook Form integrates natively with shadcn/ui's `<Form>` component, which wraps the
`FormField`, `FormItem`, `FormLabel`, `FormControl`, and `FormMessage` primitives. The integration
requires zero custom code — shadcn/ui has built it.

### Why Zod

Zod provides runtime schema validation that derives TypeScript types automatically:

```typescript
const createLeadSchema = z.object({
  firstName: z.string().min(1, 'First name is required').max(100),
  lastName: z.string().min(1, 'Last name is required').max(100),
  email: z.string().email('Invalid email address').optional().or(z.literal('')),
  source: z.enum(['website', 'referral', 'cold_call', 'event', 'other']),
  status: z.enum(['new', 'contacted', 'qualified', 'lost']).default('new'),
})

type CreateLeadInput = z.infer<typeof createLeadSchema>
// → { firstName: string; lastName: string; email?: string; source: ...; status: ... }
```

The schema serves as the single source of truth for both TypeScript types and runtime validation.
No duplication between interface definitions and validation rules.

Zod schemas can be composed and reused. A `paginationSchema` can be spread into any list query
schema. An `addressSchema` can be nested inside `contactSchema` and `companySchema`.

### How they connect

```typescript
// features/crm/components/CreateLeadForm.tsx
const form = useForm<CreateLeadInput>({
  resolver: zodResolver(createLeadSchema),
  defaultValues: {
    status: 'new',
    source: 'website',
  },
})

const mutation = useCreateLead()

function onSubmit(data: CreateLeadInput) {
  mutation.mutate(data, {
    onError: (error) => {
      // Map API field errors back to the form
      if (error.code === 'VALIDATION_ERROR' && error.fields) {
        Object.entries(error.fields).forEach(([field, message]) => {
          form.setError(field as keyof CreateLeadInput, { message })
        })
      }
    }
  })
}
```

---

## Form component pattern

Every form in the application follows the same structure:

```tsx
<Form {...form}>
  <form onSubmit={form.handleSubmit(onSubmit)}>
    <FormField
      control={form.control}
      name="firstName"
      render={({ field }) => (
        <FormItem>
          <FormLabel>First name</FormLabel>
          <FormControl>
            <Input placeholder="Jane" {...field} />
          </FormControl>
          <FormMessage />  {/* renders Zod error or server error */}
        </FormItem>
      )}
    />
    <Button type="submit" loading={mutation.isPending}>
      Create lead
    </Button>
  </form>
</Form>
```

This pattern ensures:
- Labels are properly associated with inputs (accessibility)
- Error messages are rendered consistently in the same position every time
- Loading state is tied to the mutation, not a manual `useState`

---

## Schema location convention

```
lib/validations/
├── auth.ts          → loginSchema, signupSchema, resetPasswordSchema
├── crm/
│   ├── leads.ts     → createLeadSchema, updateLeadSchema, convertLeadSchema
│   └── deals.ts     → createDealSchema, updateDealSchema
├── tasks.ts         → createTaskSchema, updateTaskSchema
└── settings/
    ├── members.ts   → inviteMemberSchema, updateRoleSchema
    └── profile.ts   → updateProfileSchema
```

Schemas live in `lib/validations/`, not inside component files. This allows them to be
imported both by the form component and by any utility that needs to validate data before
sending it to the API.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| Formik | Controlled inputs (re-renders on every keystroke); slower than RHF; no official shadcn/ui integration |
| Plain `useState` for each field | No validation, no error handling, massive boilerplate for 15-field forms |
| React Hook Form + Yup | Yup is less TypeScript-native than Zod; `z.infer<>` is cleaner than Yup's type inference |
| React Hook Form + Valibot | Valibot is smaller but Zod has a much larger ecosystem and more battle-tested integrations |
| TanStack Form | Excellent but alpha-quality; Zod integration less mature than RHF |

---

## Consequences

**Positive:**
- Uncontrolled inputs = no re-render per keystroke; forms stay fast with many fields
- Single schema = TypeScript types + runtime validation + documentation
- shadcn/ui `<Form>` primitives are drop-in and accessible
- API field errors map directly to form fields — no extra state management needed

**Negative:**
- `zodResolver` adds a small bundle weight (~14KB gzipped for Zod)
- File watcher must re-run TypeScript when schemas change (instant, but worth knowing)
- Complex conditional validation in Zod (`superRefine`, `discriminatedUnion`) has a learning curve

---

## Related decisions

- [ADR-0005](0005-ui-component-library.md) — shadcn/ui `<Form>` wraps React Hook Form natively
- [ADR-0007](0007-state-management.md) — form state stays in RHF, not in Zustand
