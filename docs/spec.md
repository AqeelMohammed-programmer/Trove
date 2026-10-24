# Personal Repository: Specification

**Status:** Accepted
**Product name:** Trove

This document describes **what the system does**, not how it is built. It is meant to be implementable in any language, framework, or SDK by a person or by an AI agent. Technology suggestions appear only in section 7 and are non-binding.

**Conventions.** The words MUST, MUST NOT, SHOULD, and MAY are used in their usual specification sense: MUST and MUST NOT are mandatory, SHOULD is a strong recommendation, MAY is optional. Rules have IDs (for example `I-2`) so an implementation can be checked against them one by one.

---

## 1. Overview

### 1.1 Purpose

A small, private repository where one person stores things and finds them again after a long time. Everything is stored as an **item** with a title, description, Markdown note, tags, and any number of **attachments** (links or files). Items are found by search, tags, and favorites.

### 1.2 Usage context

- One instance serves exactly **one user** and one private repository.
- It is delivered as a **web application** used through a browser.
- It is intended for personal use on a trusted local network, protected by a single password.
- It is designed to keep growing for years without becoming slow or messy.

### 1.3 Out of scope

These are deliberately excluded from this version:

- Multiple users, accounts, sharing, or permissions.
- Folders or collections, ratings, nested tags.
- Fetching web pages: no automatic titles, descriptions, previews, or offline copies of pages.
- Referencing files or folders by disk path. Paths may be written in an item's note, but the system never interprets them.
- Searching inside the contents of files.
- Images embedded inside notes.
- Merging two items.
- Deployment concerns such as HTTPS, containers, or hosting.

### 1.4 Terms

| Term | Meaning |
|---|---|
| Item | The one kind of thing the system stores. |
| Attachment | A link or a copied file belonging to an item. |
| Tag | A short flat label attached to items. |
| Trash | Holding area for deleted items and attachments, from which they can be restored or permanently removed. |
| Checksum | A fingerprint calculated from a file's exact content, used to detect identical files. |

---

## 2. Concepts and data model

### 2.1 Item

An item is the single top-level object. There are no separate "note", "link", or "file" types. An item with no attachments is a note-only item, and that is valid.

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | opaque unique string | yes | Assigned by the system, never reused. |
| `title` | text | yes | See rule `I-2` for the default when left empty. |
| `description` | plain text | no | Short summary. May contain line breaks. |
| `note` | Markdown text | no | Free-form content. |
| `favorite` | boolean | yes | Default `false`. |
| `tags` | set of tag names | yes | May be empty. No duplicates. |
| `attachments` | ordered list of attachments | yes | May be empty. Ordered by time added. |
| `created_at` | timestamp | yes | Set once. |
| `updated_at` | timestamp | yes | See rule `I-5`. |
| `trashed_at` | timestamp or null | yes | Null means the item is active. |

### 2.2 Attachment

An attachment belongs to exactly one item. There are two kinds.

Common fields:

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | opaque unique string | yes | Assigned by the system, never reused. |
| `kind` | `link` or `file` | yes | |
| `label` | text | no | Optional short name such as "slides" or "source". |
| `created_at` | timestamp | yes | |
| `state` | `active`, `trashed_individually`, or `trashed_with_item` | yes | See section 3.9. |
| `trashed_at` | timestamp or null | yes | |

Kind `link`:

| Field | Type | Required | Notes |
|---|---|---|---|
| `url` | text | yes | Absolute URL, stored exactly as entered after trimming surrounding whitespace. |

Kind `file`:

| Field | Type | Required | Notes |
|---|---|---|---|
| `file_name` | text | yes | Original file name. |
| `size_bytes` | integer | yes | |
| `media_type` | text | no | Detected or supplied content type. |
| `sha256` | text | yes | Lowercase hexadecimal SHA-256 of the exact file bytes. |
| content | bytes | yes | A copy stored by the system. |

### 2.3 Tag

A tag is a name. Tags are flat (no hierarchy) and free-typed (created the first time they are used). A tag exists only while at least one item (active or trashed) uses it; a tag nobody uses disappears automatically.

### 2.4 Identifiers and timestamps

- Identifiers are opaque. Implementations MAY choose any scheme that guarantees uniqueness.
- Timestamps are stored in UTC and exchanged in ISO 8601 format (for example `2026-10-24T09:30:00Z`).

---

## 3. Behavior

### 3.1 Items

- `I-1` The user creates an item through a single "add item" form with all fields: title, description, note, tags, favorite, attachments. There are no separate quick-add or per-type flows.
- `I-2` A title is required. If the title is empty or only whitespace when saving and the item has at least one attachment, the title MUST be set to the first attachment's URL (for a link) or file name (for a file). If the title is empty and there are no attachments, saving MUST be rejected with a clear message.
- `I-3` A title set by the rule in `I-2` is stored as an ordinary title. It is not recomputed later and the user can edit it.
- `I-4` An item MAY have zero attachments.
- `I-5` Any change to title, description, note, tags, or attachments sets `updated_at` to the time of the change. Toggling the favorite flag MUST NOT change `updated_at`.
- `I-6` Every field of an item can be edited after creation.
- `I-7` Deleting an item moves it to the trash (section 3.9). There is no deletion that bypasses the trash.
- `I-8` The system MUST NOT fetch web pages or contact external sites on the user's behalf.
- `I-9` Paths and folder locations are recorded by the user in the note. The system treats them as ordinary text.

### 3.2 Attachments

- `A-1` An item MAY have any number of attachments, and kinds MAY be mixed.
- `A-2` A link attachment requires a URL that is a valid absolute URL with the scheme `http` or `https`. Other schemes MUST be rejected.
- `A-3` A file attachment is created by the user providing a file. The system stores its own copy and records `file_name`, `size_bytes`, `media_type`, and `sha256`.
- `A-4` The `sha256` is calculated over the exact bytes of the file when it is added.
- `A-5` The user MAY select or drop several files at once in the add or edit form. All of them become attachments of that same item.
- `A-6` Both kinds accept an optional label.
- `A-7` Actions on an attachment:
  - Link: **open** (in a new tab) and **copy URL**.
  - File: **download**, delivering the original content under the original file name.
- `A-8` The system MUST NOT read, open, or list anything on the user's file system by path. It handles only files the user explicitly provides.
- `A-9` Removing an attachment from an item moves it to the trash individually (section 3.9).

### 3.3 Tags

- `T-1` Tags are typed freely. Typing a name not yet in use creates the tag when the item is saved.
- `T-2` While typing in a tag field, the system suggests existing tags that match what has been typed (autocomplete).
- `T-3` Tag names are trimmed, and runs of inner whitespace collapse to one space. Names are 1 to 50 characters and MUST NOT contain a comma, since the comma separates tags during input.
- `T-4` Tag names are unique ignoring case. Typing a name that matches an existing tag ignoring case MUST reuse that existing tag. The spelling shown is the one used when the tag was first created.
- `T-5` An item's tags form a set. The same tag cannot be added twice to one item.
- `T-6` The sidebar tag list and autocomplete show only tags used by at least one active (non-trashed) item.

### 3.4 Favorites

- `F-1` Each item has a favorite flag, which the user can toggle from the item list row and from the item detail view.
- `F-2` A "Favorites" view lists all active items whose flag is on.
- `F-3` The favorites filter combines with search, tag filters, and the attachment filters (section 3.5).

### 3.5 Browsing, filters, and sorting

- `B-1` The default view lists all active items, newest created first.
- `B-2` Sort orders: **newest** (by `created_at`, descending), **recently edited** (by `updated_at`, descending), and **title** (alphabetical, ignoring case). When a search is active, the default order is **relevance**, and the user MAY switch to any other order.
- `B-3` Filters are: search text, one or more tags, favorites only, has a link, has a file. All active filters combine with AND.
- `B-4` Selecting several tags means the item MUST have **all** of the selected tags.
- `B-5` "Has a link" matches items with at least one active link attachment. "Has a file" matches items with at least one active file attachment.
- `B-6` Trashed items MUST NOT appear in any normal view, search result, filter result, or tag count.
- `B-7` The list shows the total number of matching items and loads in pages or incrementally, so very large collections do not slow the screen.

### 3.6 Search

- `S-1` Search covers exactly four things: title, description, note, and tags. It does NOT cover link URLs, attachment labels, file names, or file contents.
- `S-2` Search is case-insensitive. Each word the user types MUST match the start of a word in the searchable text (prefix matching). When several words are typed, an item matches only if every word matches somewhere in its searchable text.
- `S-3` Results SHOULD be ranked by relevance, with matches in the title ranking above matches elsewhere.
- `S-4` Markdown formatting symbols in notes SHOULD NOT affect matching.
- `S-5` Search results use the same list as browsing and combine with all filters (`B-3`).
- `S-6` A saved change MUST be searchable immediately on the next query.

### 3.7 Duplicate warning when adding

- `D-1` When the user adds a link, the system checks every other link attachment, including those in the trash. If any has exactly the same URL string, the system warns the user and shows the item or items involved, marking those in the trash.
- `D-2` When the user adds a file, the system checks every other file attachment, including those in the trash, for the same `sha256`. A match produces the same kind of warning.
- `D-3` The warning is informational. The user MUST be able to proceed and save anyway.
- `D-4` A warning also applies when the match is inside the same item.
- `D-5` From the warning, the user can open a matching item, and for a trashed match, restore it.

### 3.8 Duplicate report

- `R-1` A separate section of the application, "Duplicates", shows a report. It is computed when the user opens the section, from stored data. No background jobs are required.
- `R-2` The report lists groups of two or more attachments that match:
  - Links match when their URLs are equal after removing any `#fragment` and one trailing `/` from the path. Everything else is compared exactly.
  - Files match when their `sha256` values are equal.
- `R-3` Trashed attachments and items are included, clearly marked as trashed.
- `R-4` Each group shows every attachment with its owning item, and each entry links to that item.
- `R-5` The report is read-only. The system MUST NOT delete, merge, or change anything automatically. The user resolves duplicates by sending attachments or items to the trash.
- `R-6` If no duplicates exist, the report says so.

### 3.9 Trash

- `X-1` Deleting an item sets it to trashed. Every attachment of the item that is currently `active` becomes `trashed_with_item`. Attachments already `trashed_individually` stay as they are.
- `X-2` Removing a single attachment from an active item sets its state to `trashed_individually`.
- `X-3` The trash view lists two kinds of entries: trashed items, each shown with all of its attachments beneath it, and individually trashed attachments of active items, each shown with its owning item. Entries show the date they were deleted and are ordered newest deleted first.
- `X-4` **Restore item:** the item becomes active again with its tags and favorite flag. Its `trashed_with_item` attachments become `active`. Attachments that were `trashed_individually` remain individually trashed and appear again as separate trash entries.
- `X-5` **Restore attachment:** the attachment becomes `active` again on its item.
- `X-6` **Permanently delete an item:** removes the item, all its attachments in any state, and all stored file content. It cannot be undone and requires confirmation.
- `X-7` **Permanently delete an attachment:** removes the attachment and its stored file content. It cannot be undone and requires confirmation.
- `X-8` **Empty trash:** permanently deletes everything in the trash. It requires confirmation. The system MUST NOT empty the trash automatically or expire entries over time.
- `X-9` Stored copies of files are removed only on permanent deletion, never when merely trashing.
- `X-10` Permanently deleting one attachment MUST NOT affect any other attachment, even one with identical content.
- `X-11` Trashed items keep their tags. Tags used only by trashed items are not shown (`T-6`) but reappear when an item is restored.
- `X-12` The system never deletes or changes anything outside its own storage.
- `X-13` Moving an item or attachment to the trash does not ask for confirmation, since it can be undone.

### 3.10 Tag management

- `M-1` A tag management screen lists all tags that are shown under `T-6`, each with the number of active items using it. The default order is alphabetical, and the user MAY switch to order by count.
- `M-2` **Rename:** changes a tag's name everywhere. If the new name matches another existing tag ignoring case, the rename is rejected with a message suggesting a merge. Changing only the capitalization of the same tag is allowed.
- `M-3` **Merge:** the user picks a source tag and a target tag. Every item with the source tag (active or trashed) ends up with the target tag, without duplicates, and the source tag is removed.
- `M-4` **Delete:** removes a tag from every item, active or trashed. The items themselves remain. The action shows how many items are affected and requires confirmation. It is not recoverable through the trash.
- `M-5` Rename, merge, and delete MUST NOT change `updated_at` of the affected items.

### 3.11 Scale

- `N-1` The system is designed for one person's collection growing over many years. The specification imposes no maximum number of items or maximum file size. An implementation SHOULD document any limits it has.
- `N-2` Listing, filtering, and searching SHOULD respond in under one second with at least 100,000 items on ordinary hardware.

---

## 4. Screens

All screens except first-run setup and login require an authenticated session (section 5).

### 4.1 Main screen

Three areas:

- **Top bar:** search box, an "Add item" button, and a sort dropdown.
- **Left sidebar:** navigation entries (All items, Favorites, Trash, Tags, Duplicates, Export and import, Change password, Log out) and the tag list with counts. Clicking a tag adds it to the active filters.
- **Center:** the item list, with filter controls for favorites only, has a link, and has a file, plus the currently selected tags shown as removable chips.

### 4.2 Item list rows

The list uses one row per item. Each row shows:

- The title in bold.
- A one-line snippet of the description, truncated if long.
- The tags as small chips.
- A favorite star that toggles the flag when clicked.
- The attachment count, split by kind (link count and file count) with small icons.
- The last edited date.

Clicking a row opens the item detail view.

### 4.3 Item detail view

- Title, description, and the note rendered as formatted Markdown.
- Tags (clicking one filters the list by it), favorite toggle, created and last edited dates.
- Attachments list. Each shows its label if present (otherwise the URL or file name), its kind, and for files the size. Link attachments offer open and copy; file attachments offer download.
- Buttons to edit and to move the item to the trash.

Markdown rendering rules:

- `V-1` Notes render standard Markdown (headings, emphasis, lists, code, quotes, links, tables are allowed).
- `V-2` Raw HTML inside a note MUST NOT be rendered or executed. It is shown as plain text or removed.
- `V-3` Markdown image syntax is not rendered as an inline image. It MAY be shown as plain text or as a link.
- `V-4` Links inside notes open in a new tab.

### 4.4 Add and edit form

One form with: title, description, a Markdown text area for the note, a tag input with autocomplete, a favorite checkbox, and an attachments section. In the attachments section the user can add a link (URL and optional label), add one or more files by picking or dropping them (each with an optional label), and remove an attachment. Duplicate warnings (section 3.7) appear inline. Save and cancel are always available.

### 4.5 Tag management screen

As described in section 3.10.

### 4.6 Trash screen

As described in section 3.9, with per-entry **Restore** and **Delete permanently** actions, and an **Empty trash** action.

### 4.7 Duplicates screen

As described in section 3.8.

### 4.8 Export and import screen

As described in section 6.

### 4.9 Authentication screens

First-run password setup, login, and change password (section 5).

---

## 5. Authentication

One password protects the whole instance. There are no usernames.

- `U-1` On first run, when no password has been set, the system MUST show only a password setup screen (password plus confirmation). Nothing else is accessible until a password is set.
- `U-2` The password MUST be at least 8 characters.
- `U-3` Every other screen and every data and file endpoint, including file downloads and export, MUST require a logged-in session.
- `U-4` Logging in with the correct password starts a session.
- `U-5` A session ends when the browser session ends (the user closes the browser). The session credential MUST NOT be stored persistently, and there is no "remember me". There is no inactivity timeout.
- `U-6` The user can log out at any time.
- `U-7` While logged in, the user can change the password, which requires entering the current password.
- `U-8` The password MUST be stored only as a hash produced by a slow, salted password-hashing function. The plain password MUST NOT be stored or logged.
- `U-9` Guessing protection: repeated failed login attempts MUST be slowed down or temporarily blocked (default parameters in section 8).
- `U-10` There is no password recovery inside the application. A forgotten password can be reset only by an action on the machine that runs the system. The mechanism (for example a configuration value or a command) is implementation-defined. A reset MUST NOT delete or change stored data, and SHOULD end all existing sessions.
- `U-11` Transport security and other deployment concerns are outside this specification.

---

## 6. Export and import

### 6.1 Export

- `E-1` The user can export the whole repository as **one archive file** in ZIP format.
- `E-2` The archive contains:
  - `data.json`, describing all items, tags, and attachments, including trashed ones with their trash state.
  - A `files/` directory with the content of every copied file, stored as `files/<attachment id>` with no extension.
- `E-3` The archive MUST NOT contain the password, its hash, or session information.
- `E-4` The export MUST represent a consistent snapshot of the repository at one moment.

### 6.2 `data.json` structure

```json
{
  "format": "personal-repository-export",
  "format_version": 1,
  "exported_at": "2026-10-24T09:30:00Z",
  "items": [
    {
      "id": "item-id",
      "title": "Example",
      "description": "Short summary",
      "note": "Markdown text",
      "favorite": false,
      "tags": ["reading", "rust"],
      "created_at": "2026-10-01T08:00:00Z",
      "updated_at": "2026-10-02T10:15:00Z",
      "trashed_at": null,
      "attachments": [
        {
          "id": "attachment-id-1",
          "kind": "link",
          "label": "source",
          "url": "https://example.org/article",
          "created_at": "2026-10-01T08:00:00Z",
          "state": "active",
          "trashed_at": null
        },
        {
          "id": "attachment-id-2",
          "kind": "file",
          "label": null,
          "file_name": "paper.pdf",
          "size_bytes": 123456,
          "media_type": "application/pdf",
          "sha256": "lowercase-hex-digest",
          "stored_as": "files/attachment-id-2",
          "created_at": "2026-10-01T08:05:00Z",
          "state": "trashed_individually",
          "trashed_at": "2026-10-03T12:00:00Z"
        }
      ]
    }
  ]
}
```

Tags appear only as names inside each item. All timestamps are UTC ISO 8601. Unknown extra fields SHOULD be ignored on import so later versions can add fields.

### 6.3 Import

- `E-5` Import is **restore-only**: it is allowed only into an empty repository (no items, no tags, empty trash). If the repository is not empty, import MUST be refused with a clear message. Merging into existing data is not supported in this version.
- `E-6` Before changing anything, the system validates the whole archive: the `format` and `format_version` are recognized, `data.json` is well formed, every referenced file exists in the archive, and each file's SHA-256 matches its `sha256` value. If any check fails, nothing is imported.
- `E-7` Import is all-or-nothing. A failure part-way MUST leave the repository empty.
- `E-8` Import preserves identifiers, timestamps, trash states, favorites, tags, labels, and file content.
- `E-9` Import does not change the password.
- `E-10` Exporting and then importing into a fresh instance MUST produce a repository identical in everything the user can see.

---

## 7. Implementation notes (non-binding)

Nothing in this section is required. It records the ideas discussed while writing the spec.

- A single-user web app can keep metadata in an embedded database such as **SQLite** (one file, no server) with its built-in full-text search (**FTS5**) for `S-1` to `S-6`. A server database such as PostgreSQL would also work but is not needed for one user.
- Copied files can live in a data folder on disk, named by attachment id, with the database holding only metadata.
- Compute the SHA-256 while the file is being received, so large files do not need to be held fully in memory.
- Use a well-reviewed Markdown library together with an HTML sanitizer to satisfy `V-2`.
- Use a standard password-hashing function such as Argon2id, bcrypt, or scrypt, and a session-scoped cookie for `U-5`.
- Because attachment identity is by `id`, an implementation may share one stored copy among identical files or keep separate copies, as long as `X-10` holds.
- A native desktop build is a possible alternative target. If a Rust toolkit such as Slint is used, note that its Markdown support covers only a subset of syntax, so check `V-1` against it first.

---

## 8. Open decisions and defaults

### 8.1 Open decisions

1. **Other URL schemes.** Only `http` and `https` are allowed (`A-2`). Allowing `mailto:`, `ftp:`, or others is undecided.
2. **Attachment order.** Attachments are ordered by time added. Manual reordering is not part of this version.

### 8.2 Defaults chosen for details not discussed

Change any of these before building.

| Area | Default |
|---|---|
| Length limits | Title 300 characters, description 5,000, note 500,000, label 100, tag name 50. |
| Minimum password length | 8 characters. |
| Guessing protection | After 5 consecutive failed logins, block further attempts for 60 seconds. Each additional 5 failures doubles the block, up to 15 minutes. A successful login resets the count. |
| Tag matching | Case-insensitive, with the first-created spelling displayed. |
| Unused tags | A tag with no items (active or trashed) disappears automatically. |
| `updated_at` | Changes with content edits only. Not changed by favorite toggles or by tag rename, merge, or delete. |
| Search matching | Word-prefix, all words required, title matches ranked highest. |
| Add-time duplicate check | Exact URL string match and exact SHA-256 match. |
| Duplicate report matching | Links also match when they differ only by a `#fragment` or one trailing slash. |
| Confirmations | Required for permanent deletion, empty trash, and tag delete. Not required for moving to the trash. |
| List loading | Paged or incremental, default page size 50. |
| Performance target | Under one second for list and search with 100,000 items. |
| Trash order | Newest deleted first. |
| Notes | Raw HTML is never rendered. Image syntax is not rendered inline. |
| Export content | Excludes the password, its hash, and sessions. |
| Search scope | Title, description, note, tags only. Link URLs, labels, and file names are not searched, but this is easy to extend later. |
