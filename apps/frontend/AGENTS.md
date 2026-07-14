# Frontend Instructions

These rules extend the repository-level `AGENTS.md` for files under
`apps/frontend/`.

## Stack

SvelteKit 2 SSR application using Svelte 5 runes, strict TypeScript, Vite,
`@sveltejs/adapter-node`, Tailwind v4, DaisyUI 5, and `@lucide/svelte`.

## Commands

Run from `apps/frontend/`:

```sh
npm run dev
npm run check
npm run lint
npm test
npm run build
```

Use `BACKEND_URL=http://localhost:8080 npm run dev` to target a locally
forwarded backend.

After `npm run build`, run the production SSR bundle locally with:

```sh
BACKEND_URL=http://localhost:8080 node build
```

## Data Flow

- Reads use server `load` functions in `+page.server.ts`.
- Writes use form actions in `+page.server.ts` with
  `<form method="POST" use:enhance>`.
- Client-driven pagination uses a per-list `+server.ts` endpoint returning JSON.
- Do not add a generic API proxy or fetch data on component mount.
- Server-side reads and writes go through `apiClient(event)` in
  `src/lib/server/api/client.ts`, which resolves backend-relative paths against
  `BACKEND_URL` and forwards the session cookie, plus `X-Forwarded-For` (from
  `event.getClientAddress()`) so login/register throttling sees the real client
  IP. These calls run in the Node server, never the browser, so they never
  involve CORS.
- The browser must not call the backend directly; it talks only to this
  SvelteKit server. CSP `connect-src 'self'` enforces the boundary, keeps the
  session cookie `httpOnly`, keeps the backend API off the public surface, and
  contains XSS exfiltration to this origin.
- Use the central `unwrap<T>()` helper in `src/lib/server/api/http.ts` for API
  responses. It handles `204 No Content` and non-JSON error bodies; do not call
  `response.json()` directly.

## Svelte and TypeScript

- Keep strict TypeScript enabled. Prefer `unknown` over `any` and map transport
  DTOs deliberately.
- Do not add `eslint-disable`, `@ts-ignore`, `@ts-expect-error`, or similar
  suppressions to get checks passing. Fix the underlying issue unless a
  documented external API, generated code, or tooling false positive requires a
  narrow exception.
- Use Svelte 5 runes and SvelteKit primitives already established in the
  codebase.
- Keep route-specific server behavior in route server files and reusable server
  API access under `src/lib/server/api/`.
- Use behavior-oriented Vitest tests for mappers, parsing, pagination state, API
  handling, and other non-trivial logic.

## UI Conventions

- Prefer DaisyUI components and Tailwind utilities in templates.
- Configure themes CSS-first through `@plugin "daisyui/theme"` in `src/app.css`;
  use `@theme` for custom tokens.
- Use `@lucide/svelte` icons. Add inline SVG only when Lucide cannot represent
  the symbol.
- Use intentional widths: `max-w-xl` for auth, settings, feed, and single-post
  flows; `max-w-5xl` for profile grids, upload creation, and app-shell
  alignment.
- Do not add a dependency when the platform, SvelteKit, or an existing project
  primitive handles the requirement. Remove direct dependencies that are no
  longer imported.

## Images and Uploads

- Resize large JPEG, PNG, GIF, and WEBP files before upload, targeting less than
  900 KB. The backend still enforces the 1 MB hard limit.
- Upload the file first, then create the image record with `POST /images` using
  the returned filename.
- Image bytes are served same-origin by `src/routes/uploads/[key]/+server.ts`,
  which streams them from the backend through `apiClient` without buffering.
- Use `imageUrl()` for stored image paths. CSP keeps
  `img-src 'self' data: blob:` so local upload previews can render `data:` URLs
  and client-side resizing can decode selected files through `blob:` object
  URLs.

## SSR and Browser Security

- `@sveltejs/adapter-node` runs the production application as a Node server on
  port 8080.
- Security headers are set in `src/hooks.server.ts`.
- CSP uses nonce mode configured in `svelte.config.js`. Do not introduce inline
  scripts without the request nonce.
- Never render user-controlled HTML directly. Validate user-controlled `href`
  and `src` values against an explicit scheme and origin policy.
- The theme controller (`$lib/theme.svelte.ts`) writes to localStorage and a
  cookie. The cookie is read by `hooks.server.ts`, which uses
  `transformPageChunk` to inject the resolved theme into the SSR HTML before
  first paint, preventing FOUC without an inline script. `+layout.server.ts`
  parses the cookie so the initial `ThemeMode` is correct for server-rendered
  UI. `+layout.svelte` calls `setThemeContext` and starts the client-side
  controller (system mode resolution, cross-tab sync) in `onMount`.
- The container runs as uid 1000 with a read-only root filesystem. The health
  probe is `/health`.
