# Frontend

## Stack

SvelteKit with Svelte runes, `@sveltejs/adapter-node`, Tailwind, DaisyUI,
`@lucide/svelte`, strict TypeScript.

## Route Groups and Guards

```
/                           → redirect 303 → /feed
└── (app)/                  +layout.server.ts: GET /users/me → currentUser: User | null, no redirect; unread count only when authenticated
    ├── login/              form action: POST /sessions; public
    ├── register/           form action: POST /users then POST /sessions; public
    ├── feed/               load: GET /feed (authenticated) or GET /posts/popular (anonymous); public read
    ├── posts/[publicId]/   load: GET /posts/{id} + GET /posts/{id}/comments; public read, like/comment gated to login
    ├── [username=username]/ load: GET /users/{username} + GET /users/{username}/posts; public read, follow gated to login
    │   ├── likes/          load: GET /users/{username}/likes; public read
    │   └── [mode=connections]/ load: followers or following list; public read, follow gated to login
    └── (private)/          +layout.server.ts: redirect /login if currentUser absent
        ├── notifications/      load: GET /notifications + PUT /notifications/{id}/read (mark all unread as read)
        ├── search/             load: GET /search?q=&type=users|posts|hashtags (3x in parallel, small preview limit) → unified grouped sections, no tabs; empty query shows discovery content: suggested users via GET /users/suggested + popular posts grid via GET /posts/popular; live dropdown-as-you-type calls GET /suggest (users, hashtags) + GET /search?type=posts&limit= (posts preview)
        ├── upload/             form action: POST /uploads → POST /posts
        ├── suggest/            GET +server.ts — typeahead proxy: GET /users/search or /hashtags/search
        ├── logout/             form action: DELETE /sessions → delete session cookie → redirect /login (cookie deleted even if backend call fails)
        └── settings/           layout → redirect to sub-routes
            ├── profile/        form action: PUT /users/{id}
            ├── password/       form action: PUT /users/{id}
            └── sessions/       load: GET /sessions; revoke action: DELETE /sessions/{sessionId}
```

## Layout Hierarchy

```
+layout.svelte (root)
  - loads: theme from cookie
  - renders: navigation progress bar + {children}

  (app)/+layout.svelte
    - loads: currentUser (GET /users/me) → User | null, no redirect; unreadCount from GET /notifications, only when authenticated
    - renders: <Navbar currentUser unreadCount> + <main>{children}</main>
    - width: max-w-5xl px-4 pb-8 pt-4

  (app)/(private)/+layout.server.ts
    - loads: nothing — reads currentUser from parent(); redirect /login if absent
    - guards: upload/, suggest/, logout/, settings/, notifications/, search/
```

## Route Parameters and Matchers

| Param      | Matcher                     | Pattern                                  |
| ---------- | --------------------------- | ---------------------------------------- |
| `username` | `src/params/username.ts`    | `^@[a-z0-9._]{3,30}$` (with leading `@`) |
| `mode`     | `src/params/connections.ts` | `followers` or `following`               |
| `publicId` | none                        | UUID validated in backend handler        |

`stripAt()` removes the leading `@` before passing the username to backend API
calls.

## Data Fetching Strategy

| Pattern                     | When used                                                            |
| --------------------------- | -------------------------------------------------------------------- |
| `+page.server.ts` `load`    | Initial page data — runs server-side                                 |
| `+page.server.ts` `actions` | All mutations — POST form actions with `use:enhance`                 |
| `+server.ts` `GET`          | Client-driven pagination "load more" — returns JSON                  |
| `createPagination()`        | Client-side state for progressive list loading                       |
| `fetchCursorPage()`         | Browser-side cursor URL construction and JSON parsing for pagination |

No data is fetched on component mount. The browser never calls the backend
directly.

Backend error bodies are not rendered as UI copy. Server API helpers preserve
HTTP status codes for control flow, then map failures to frontend-owned messages
before they can reach SvelteKit error pages, form failures, or pagination state.

## SSR Boundary

Everything runs in the Node server. `apiClient(event)` resolves backend paths
against `BACKEND_URL` env var and forwards the session cookie. These requests
are server-to-server and never cross CORS.

`hooks.server.ts` sets browser security headers for every response and adds
`Strict-Transport-Security` when the public request URL is HTTPS.

Browser → SvelteKit server: form POST or page navigation. Browser fetches for
pagination: `GET /feed`, `GET /@{username}`, `GET /posts/{id}/comments`,
`GET /search` — all route to SvelteKit `+server.ts` handlers, which call the
backend server-side.

## Image Proxy

`GET /uploads/[key]` in `src/routes/uploads/[key]/+server.ts`:

- Validates `key` against `^[A-Za-z0-9._-]{1,255}$` and rejects `..` traversal.
- Streams response body directly from backend without buffering.
- Forwards: `content-type`, `content-length`, `etag`, `last-modified`,
  `cache-control`.

## Upload Actions

Post image and avatar form actions validate that submitted files are JPEG, PNG,
GIF, or WEBP images and no larger than 1 MB before forwarding them to the
backend. The production SvelteKit server sets `BODY_SIZE_LIMIT=1100K` so the 900
KB client-side resize target plus multipart overhead reaches the action handler
for explicit validation.

## Key Frontend Routes (API Endpoints)

| Path                         | Method | Handler           | Backend call                                              |
| ---------------------------- | ------ | ----------------- | --------------------------------------------------------- |
| `/feed`                      | GET    | page load         | GET /feed                                                 |
| `/feed`                      | GET    | +server.ts        | GET /feed?cursor=                                         |
| `/notifications`             | GET    | page load         | GET /notifications + PUT /notifications/{id}/read         |
| `/notifications`             | GET    | +server.ts        | GET /notifications?cursor= + PUT /notifications/{id}/read |
| `/search`                    | GET    | page load         | GET /search?type=users,posts,hashtags (parallel, limit=5)  |
| `/search`                    | GET    | +server.ts        | GET /search?cursor= (per-section "Load more"), or ?limit= (dropdown posts preview) |
| `/suggest`                   | GET    | +server.ts        | GET /users/search or /hashtags/search (dropdown users/hashtags preview) |
| `/@{username}`               | GET    | page load         | GET /users/{u} + GET /users/{u}/posts                     |
| `/@{username}`               | GET    | +server.ts        | GET /users/{u}/posts?cursor=                              |
| `/@{username}/likes`         | GET    | page load         | GET /users/{u}/likes                                      |
| `/@{username}/likes`         | GET    | +server.ts        | GET /users/{u}/likes?cursor=                              |
| `/@{username}/{mode}`        | GET    | page load         | GET /users/{u}/followers or /following                    |
| `/@{username}/{mode}`        | GET    | +server.ts        | GET /users/{u}/followers or /following?cursor=            |
| `/posts/{id}/comments`       | GET    | +server.ts        | GET /posts/{id}/comments?cursor=                          |
| `/settings/sessions`         | GET    | page load         | GET /sessions                                             |
| `/settings/sessions?/revoke` | POST   | named form action | DELETE /sessions/{sessionId}                              |
| `/uploads/[key]`             | GET    | +server.ts        | GET /uploads/{key} (proxied)                              |
| `/health`                    | GET    | +server.ts        | returns `ok` text                                         |

## Pagination Helpers

`createPagination` state: `items`, `cursor`, `loading`, `error`. Resets when
`getInitial()` returns a new array reference (client-side navigation). `more()`
appends and advances cursor. Used in feed, profile, liked posts, connections,
search, and comment lists. Browser pagination fetches use `fetchCursorPage()` so
cursor encoding, query-string composition, and client error messages stay
centralized.

## Session Cookie Relay

On login, the backend sets `Set-Cookie: session=...` on its own origin.
`applySessionCookie()` in `auth.ts` accepts only a 28-character unpadded
base64url token (`[A-Za-z0-9_-]{28}`), parses the `Set-Cookie` header, and
re-emits it on the SvelteKit origin with `HttpOnly`, `Secure`,
`SameSite=Strict`, `Path=/`, and the backend-provided `Max-Age`. The cookie is
then included in all subsequent `apiClient` calls via
`event.cookies.get('session')`.

## Active Sessions

`/settings/sessions` follows the BFF boundary. Its server load calls
`GET /sessions` through `apiClient`, maps the API timestamps to `Date` values,
and renders the initial list during SSR; the browser does not fetch sessions on
mount.

Remote revocation uses the enhanced named `revoke` form action. The action
validates the submitted public UUID, calls
`DELETE /sessions/{encodeURIComponent(sessionId)}` through `apiClient`, and maps
backend failures to safe UI messages. The page removes a session from its local
list only after a successful response. The current session has no revoke control
and must be terminated through the existing Logout action.

## Type Mapping

DTOs (`UserDto`, `PostDto`, `CommentDto`, `SessionDto`) are mapped to domain
types (`User`, `Post`, `Comment`, `Session`) via `mappers.ts`. Timestamp strings
are deliberately converted to `Date` values; sessions map both `created` and
`expiresAt`.
