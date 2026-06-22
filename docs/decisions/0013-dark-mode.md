# ADR-0013: Theme — dark/light mode with next-themes

**Date:** 2025-06-22
**Status:** Accepted
**Deciders:** Mridha

---

## Context

BusinessSAAS users must be able to switch between dark and light mode. The toggle is user-controlled
and must persist across sessions. The selected theme must be applied immediately on page load,
without a flash of the wrong theme (known as FOUC — Flash of Unstyled Content).

Requirements:
- System preference detection (`prefers-color-scheme`)
- Manual override that persists across browser sessions
- Zero FOUC — correct theme rendered on first paint, before React hydrates
- Works with Tailwind CSS v4's CSS variable-based dark mode
- Minimal JavaScript overhead

---

## Decision

Use **next-themes** for theme management.

Tailwind CSS v4's design tokens provide the actual color values via CSS variables. next-themes
handles the class application and persistence. The two work together: next-themes adds the `dark`
class to `<html>`, Tailwind reads it to activate the `dark:` variant.

---

## How it works

### 1. CSS variables define all colors

```css
/* app/globals.css */
@theme {
  --color-background: #ffffff;
  --color-foreground: #0a0a0a;
  --color-primary: #4f46e5;
  --color-muted: #f4f4f5;
  --color-muted-foreground: #71717a;
  --color-border: #e4e4e7;
  --color-card: #ffffff;
  /* ... complete token set */
}

.dark {
  --color-background: #0a0a0a;
  --color-foreground: #fafafa;
  --color-primary: #6366f1;
  --color-muted: #27272a;
  --color-muted-foreground: #a1a1aa;
  --color-border: #3f3f46;
  --color-card: #18181b;
}
```

When next-themes adds `class="dark"` to `<html>`, the `.dark` CSS block activates and all
variables switch to their dark values. Every component using these variables updates instantly
— no JavaScript re-render required.

### 2. next-themes provides the class

```tsx
// app/layout.tsx
import { ThemeProvider } from 'next-themes'

export default function RootLayout({ children }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          {children}
        </ThemeProvider>
      </body>
    </html>
  )
}
```

`suppressHydrationWarning` on `<html>` suppresses the React hydration mismatch warning that
occurs because next-themes modifies the `class` attribute on the server vs client. This is
expected and safe — it is the mechanism that prevents FOUC.

`defaultTheme="system"` means the initial theme follows `prefers-color-scheme`. If the user
has set a preference in the OS, it's respected automatically.

`disableTransitionOnChange` prevents color transitions from firing when the theme switches
(which looks janky if all elements animate colour simultaneously).

### 3. User toggle

```tsx
// components/layout/ThemeToggle.tsx
import { useTheme } from 'next-themes'
import { Sun, Moon, Monitor } from 'lucide-react'

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon">
          <Sun className="h-4 w-4 rotate-0 scale-100 dark:-rotate-90 dark:scale-0 transition-all" />
          <Moon className="absolute h-4 w-4 rotate-90 scale-0 dark:rotate-0 dark:scale-100 transition-all" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => setTheme('light')}>
          <Sun className="mr-2 h-4 w-4" /> Light
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          <Moon className="mr-2 h-4 w-4" /> Dark
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          <Monitor className="mr-2 h-4 w-4" /> System
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
```

### 4. Persistence

next-themes persists the user's choice in `localStorage` under the key `theme`. On the next
page load, the stored preference is applied before React renders — this is the FOUC prevention
mechanism.

---

## FOUC prevention mechanism

Without special handling, the page would:
1. Render with the default (light) theme
2. JavaScript loads
3. React reads localStorage, finds `"dark"`
4. Applies dark class
5. Page flickers from light to dark

next-themes prevents this by injecting an inline `<script>` at the top of `<head>` that runs
synchronously before any content renders:

```html
<script>
  // (simplified) next-themes injects this
  const stored = localStorage.getItem('theme')
  const system = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  const theme = stored ?? 'system'
  document.documentElement.classList.add(theme === 'system' ? system : theme)
</script>
```

This script runs before the browser paints anything. The correct class is set before any
CSS is applied, so there is no flash.

---

## Alternatives considered

| Option | Reason rejected |
|--------|----------------|
| `prefers-color-scheme` media query only | No user override; system preference only |
| Manual localStorage + useEffect | FOUC on page load; complex to implement correctly |
| CSS-only `color-scheme: dark light` | No JavaScript control; cannot be overridden per user preference |
| Tailwind `darkMode: 'media'` | Cannot override system preference; user toggle impossible |

---

## Consequences

**Positive:**
- Zero FOUC — correct theme applied before first paint
- System preference respected by default
- User override persists in localStorage
- Theme switch is instant — CSS variable change, no JavaScript re-render
- `next-themes` is tiny (~3KB) and the most widely used solution for this in Next.js

**Negative:**
- `suppressHydrationWarning` on `<html>` hides the expected hydration mismatch — acceptable
- `localStorage` usage means the preference does not sync across devices (not required)
- `disableTransitionOnChange` means theme switch is instantaneous, not animated — correct UX for dashboards

---

## Related decisions

- [ADR-0005](0005-ui-component-library.md) — Tailwind CSS v4 CSS variables are the color source
- [ADR-0004](0004-frontend-framework.md) — Next.js App Router; `ThemeProvider` wraps in root layout
