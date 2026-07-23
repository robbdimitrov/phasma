# Design System

## Theme Structure

Themes are defined CSS-first in `src/app.css` using `@plugin "daisyui/theme"`.
The `data-theme` attribute on `<html>` selects the active theme. Theme selection
persists via a `theme` cookie (1-year max-age, SameSite=Lax) and `localStorage`;
the cookie is read in a nonce-bearing `<script>` in `app.html` before paint to
prevent FOUC.

### Light Theme (default)

| Token                     | Value     |
| ------------------------- | --------- |
| `--color-primary`         | `#ff4a85` |
| `--color-secondary`       | `#8b5cf6` |
| `--color-accent`          | `#06b6d4` |
| `--color-neutral`         | `#020617` |
| `--color-neutral-content` | `#ffffff` |
| `--color-base-100`        | `#ffffff` |
| `--color-base-200`        | `#f1f5f9` |
| `--color-base-300`        | `#e2e8f0` |
| `--color-base-content`    | `#020617` |

### Dark Theme

| Token                     | Value     |
| ------------------------- | --------- |
| `--color-primary`         | `#ff4a85` |
| `--color-secondary`       | `#a78bfa` |
| `--color-accent`          | `#22d3ee` |
| `--color-neutral`         | `#ffffff` |
| `--color-neutral-content` | `#020617` |
| `--color-base-100`        | `#020617` |
| `--color-base-200`        | `#151d30` |
| `--color-base-300`        | `#1e293b` |
| `--color-base-content`    | `#ffffff` |

`neutral` is an inverted accent for solid buttons; its polarity flips per theme.

### Custom Tokens (`@theme`)

| Token                  | Value                               | Use                                |
| ---------------------- | ----------------------------------- | ---------------------------------- |
| `--shadow-glass`       | `0 8px 32px 0 rgba(31,38,135,0.07)` | Glassmorphism card shadow (light)  |
| `--shadow-glass-dark`  | `0 8px 32px 0 rgba(0,0,0,0.37)`     | Glassmorphism card shadow (dark)   |
| `--shadow-glass-glow`  | `0 0 15px rgba(255,51,102,0.2)`     | Primary glow effect                |
| `--animate-like-pop`   | `like-pop 220ms ease-out`           | Heart like animation               |
| `--animate-like-burst` | `like-burst 700ms ease-out`         | Double-click-to-like image overlay |

## Browser Titles

User-facing pages use `Page - Phasma` document titles. The root redirect has
no user-visible title; home/feed uses `Feed - Phasma`.

## Component Inventory

### `Navbar`

Fixed-height pill header (`h-16`, `rounded-full`, backdrop blur). Contains app
logo and primary navigation links (Home, Search, Upload, Profile). Active state:
white background + shadow on the icon pill. The notifications bell shows a
small `bg-primary` dot when `unreadCount > 0`; it does not show a count.

### Notifications List (`/notifications`)

Each row's leading icon is color-coded by notification type: `like` uses a
rose `Heart` icon (`bg-rose-500/20 text-rose-500`), `comment` and `follow` use
a primary-tinted `MessageCircle`/`UserPlus` icon (`bg-primary/20 text-primary`).
Read notifications keep the same type-colored badge and use subtle text dimming;
the small `bg-primary` dot is the only explicit unread-state marker.

### `Avatar`

Circular image link to `/@{username}`. Wraps `imageUrl()` for fallback. Props:
`username`, `avatar`, `size` (default `h-11 w-11`).

### `PostCard`

Full-width card (`rounded-2xl`, `border-base-300`, `bg-base-100`). Two modes:

- Default (feed): shows image, like button with `animate-like-pop`, comment
  count link, description (linkified), timestamp.
- `singleView=true`: shows public post details; signed-in viewers also get the
  comment input form, comment list pagination, and delete buttons per own
  comments.
- Owner sees a delete button; triggers a confirmation modal (`role="dialog"`,
  `aria-modal`).
- Optimistic like/unlike with rollback on failure.
- Double-clicking the image likes the post (no-op if already liked or logged
  out) and plays a large heart `animate-like-burst` overlay on the image.

### `ImageTile`

Square aspect-ratio image link to a post detail page (`/posts/{postId}`).
Renders an optional `children` snippet as an absolutely-positioned overlay.
Base primitive for post grid tiles.

### `Thumbnail`

Wraps `ImageTile`, adding a like-count overlay on hover. Takes a structural
`ThumbnailPost` (`publicId`, `filename`, `description`, `likes`) rather than
the full `Post` model, so any post-like DTO can be passed without adaptation.
Used in profile grids, the search page's discovery (popular posts) grid, and
(via a field-renaming literal) the search page's query-results grid.

### `SearchPostThumbnail`

Wraps `Thumbnail`, mapping `SearchPostItem`'s `id` to `publicId`. Used for the
search page's query-results grid, once a search is active.

### Form Controls

Text inputs and textareas use DaisyUI v5 `input`/`textarea` without legacy
`*-bordered` classes. Focus state is a single high-contrast control border with
no offset halo; embedded action icons use opaque strokes so overlapping paths do
not darken at intersections.

### `ProfileHeader`

Horizontal card with avatar, display name, `@username`, bio (linkified),
post/like/follower/following counts linking to the relevant pages; likes and
connection-list links require a signed-in session. Current user sees Settings
link; others see Follow/Unfollow button with optimistic state and rollback.
Takes a required `active: 'posts' | 'likes' | 'followers' |
'following'` prop; the matching stats-row link gets a persistent (non-hover)
primary text color so it reads as the current section, matching the idle
stats' bold-number/muted-label contrast (full `text-primary` on the count,
`text-primary/70` on the label). This stats row is the sole cross-section
navigation for the profile shell — there is no separate tabs strip on the
Posts, Likes, or Followers/Following subpages.

### `Linkified`

Inline `<span>` that parses text into `mention`, `hashtag`, `url`, and `text`
tokens and renders links. Mentions link to `/@{username}`, hashtags link to
`/search?q=%23{tag}`.

### `LoadMoreButton`

Loading-state-aware button that calls `createPagination().more()`.

### `EmptyState`

Placeholder UI for empty lists.

## Layout Widths

| Context                              | Width                                   |
| ------------------------------------ | --------------------------------------- |
| Auth pages (login, register)         | `max-w-xl`                              |
| Settings pages                       | `max-w-xl`                              |
| Feed                                 | `max-w-xl`                              |
| Single post view                     | `max-w-xl`                              |
| Profile page grid, upload, app shell | `max-w-5xl`                             |
| Connections (followers/following)    | `max-w-5xl` (list itself is `max-w-xl`) |

## Icons

All icons from `@lucide/svelte`. Inline SVG is not used.
