# User interface and design system

This page explains how Trove looks and why. It records the colour, type and
spacing decisions so later changes stay consistent, and it documents the
light/dark theme preference. Behaviour itself lives in
[`../spec.md`](../spec.md); this page does not restate the screens.

## Principles

- **Name colour by role, never by hue.** Roles are `text`, `secondary-text`,
  `border`, `input-border`, `background`, `surface`, `link`, `focus`,
  `danger`, `success`. Components reference the role, so a palette change does
  not touch components. This follows the GOV.UK Design System approach.
- **One accent, used only for links and the primary action.** Everything else
  is neutral. This is what keeps the interface calm.
- **Contrast is measured, not guessed.** Every text and UI pair meets
  [WCAG 2.2](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)
  AA: 4.5:1 for body text and 3:1 for large text and meaningful UI.
- **No build step, no framework, no web fonts.** One hand-written stylesheet
  (`internal/web/static/style.css`) and vendored assets only, per `AGENTS.md`
  section 3.

## Colour

Token names and their light/dark values. Ratios are contrast against the page
background.

| Role | Light | Ratio | Dark | Ratio |
|---|---|---|---|---|
| `--ink` (primary text) | `#1a1a18` | 17.2:1 | `#e9e9e6` | 14.9:1 |
| `--ink-secondary` | `#5a5a56` | 6.9:1 | `#b8b8b3` | 9.1:1 |
| `--ink-muted` | `#767670` | 4.6:1 | `#9a9a95` | 6.4:1 |
| `--border` (hairline) | `#e3e3df` | — | `#2c313a` | — |
| `--border-strong` (inputs) | `#8a8a83` | 3.5:1 | `#727a86` | 3.3:1 |
| `--accent` (links, primary) | `#0f766e` | 5.5:1 | `#6fbec2` | 8.4:1 |
| `--danger` | `#a3281c` | 8.1:1 | `#e0907f` | 7.7:1 |

The accent is **teal** by default. Two alternatives were measured and pass the
same checks; switching is a one-line change to `--accent` (and its dark value):

| Palette | Light | Dark |
|---|---|---|
| Teal (default) | `#0f766e` | `#6fbec2` |
| Blue | `#1d70b8` | `#7fb2e5` |
| Burgundy | `#8c2f39` | `#e0959c` |

Surfaces (`--surface`, `--surface-2`) and the page (`--paper`) are near-neutral
so they stay quiet; only the accent carries colour.

## Type

Serif for headings and note prose (reading), system sans for the interface.
Sizes and line-heights are fixed points on a scale; line-heights are multiples
of 5px for a stable vertical rhythm.

| Token | Size / line-height | Used by |
|---|---|---|
| `--t-xs` | 13 / 20 | labels, hints, chips, counts |
| `--t-sm` | 16 / 20 | controls, list meta, buttons |
| `--t-md` | 17 / 25 | body text |
| `--t-lg` | 20 / 30 | item titles, subtitles |
| `--t-xl` | 24 / 30 | `h2` |
| `--t-2xl` | 32 / 40 | page titles on sub-screens |
| `--t-3xl` | 40 / 48 | `h1` |

Note prose renders at 18px / 1.7 with a measure of about 34em (roughly 66–74
characters), inside the 45–90 character range recommended by
[Practical Typography](https://practicaltypography.com/summary-of-key-rules.html).

## Spacing

A single 4-based scale (`--s1` … `--s9`, i.e. 4/8/12/16/20/24/32/40/48px) is
used for margins, padding and gaps, so spacing is systematic rather than ad
hoc. Hairline borders (`--border`) separate list rows and sections instead of
boxes; bordered cards are reserved for the trash, duplicate groups and the auth
form.

## Layout

- Sticky top bar: brand, search, primary "Add item", and the theme switch.
- Left sidebar: navigation and the tag list with counts. Collapses above the
  content on narrow screens.
- Reading column: the item detail caps at about 46rem; note prose caps at 34em.

## Focus and accessibility

- Focus is a solid 2px ring in the primary ink colour with a 2px offset, so it
  is visible on every surface, in both themes.
- Every control has a visible label; icon-only buttons (favourite) carry
  `aria-label` and `aria-pressed`, and decorative SVG is `aria-hidden`.
- Semantic HTML, a skip link, and full keyboard operation.
- Dark mode is applied from `prefers-color-scheme` by default.

## Theme preference

A small **Auto / Light / Dark** switch lives in the top bar. It is a UI
preference, not a behaviour the specification defines.

- `internal/web/static/theme.js` loads synchronously in `<head>` and applies the
  saved choice before the page paints, so there is no flash of the wrong theme.
- `internal/web/static/app.js` wires the buttons and reflects the current state
  with `aria-pressed`.
- The choice is stored in `localStorage` under `trove-theme`. "Auto" removes the
  key and follows the operating system.
- Both scripts are external files because the Content-Security-Policy forbids
  inline script (see [security](security.md)). With JavaScript disabled the
  switch does nothing and the interface follows the system setting.

## Related pages

- [Overview](overview.md)
- [Security](security.md)
- [Development guide](../guides/development.md)
