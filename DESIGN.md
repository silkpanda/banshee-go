# Design Tokens & Guidelines — banshee

Visual direction derived from the Raycast entry in `awesome-design-md` (tactical near-black desktop chrome, hairline `1px` borders, tight radius) paired with *Banshee* alarm telemetry accents.

## Foundations

### Typography
- **Display / Headings:** `Hanken Grotesk`, weights `500`, `600`, `700` (clean geometric sans; avoids overused Inter/Space Grotesk defaults)
- **Body / Interface:** `Hanken Grotesk`, weights `400`, `500`
- **Code / Telemetry / Monospace:** `JetBrains Mono`, weights `400`, `500` (used for timestamps, `seq` cursors, relay URLs, and status badges)

### Color Palette
- **Canvas / Background:** `#07080a` (`--background`)
- **Surface / Panel:** `#0d0d0d` (`--popover` / secondary panel)
- **Surface / Card:** `#121212` (`--card`)
- **Primary / Brand Action:** `#f4f4f6` (`--primary`)
- **On Primary (Text):** `#07080a` (`--primary-foreground`)
- **Border / Hairline:** `#242728` (`--border`)
- **Muted / Secondary Text:** `#9c9c9d` (`--muted-foreground`)
- **Alarm / Banshee Crimson:** `#ff5757` (primary active alarm accent) / `rgba(255, 97, 97, 0.15)` (soft alarm tint)
- **Relay Healthy (Emerald):** `#59d499` / `rgba(89, 212, 153, 0.15)`
- **Relay Degraded (Amber):** `#ffc533` / `rgba(255, 197, 51, 0.15)`
- **Info / Accent Blue:** `#57c1ff` / `rgba(87, 193, 255, 0.15)`

## Surface & Hierarchy
- **Tonal Ladder:** Canvas `#07080a` → Surface `#0d0d0d` → Card `#121212`, separated strictly by `1px` hairline borders (`#242728`) rather than heavy drop shadows.
- **Radius:** Tight `8px` (`0.5rem`) on cards and panels, `6px` on inputs and buttons, full pill on status badges.
- **Card Nesting Rule:** Never nest cards inside cards. Use `Separator` or subtle spacing (`gap-3` / `gap-4`) inside a single card instead.
- **Dark-Mode Native:** Always keep `color-scheme: dark` on `:root` so WebView2 scrollbars and native controls match the dark surface.

## Component Conventions
- **Base Primitives:** `shadcn/ui` configured with **Base UI** (`base-nova` preset) + `craft` layout primitives (`ds.tsx`) + `lucide-react` icons.
- **Semantic Tokens Only:** Components use semantic Tailwind classes (`bg-card`, `text-foreground`, `text-muted-foreground`, `border-border`) mapped in `frontend/src/style.css`—never raw arbitrary colors scattered inside JSX.
- **Motion:** Crisp, functional transitions (`150ms–200ms` ease-out); no bouncy or elastic easing. Reserve pulsing glow solely for live unacknowledged alarms.
- **Verification:** Run `/impeccable detect`, `/impeccable audit`, and `/impeccable polish` after UI changes.
