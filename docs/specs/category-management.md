# Spec — Category CRUD + persisted color (`afwilto-certifications-mngr`)

**Status:** Draft (proposed 2026-09-28)
**App (bounded context):** `backend/afwilto-certifications-mngr` (Go + Postgres)
**Also touches:** `frontend/apps/afwilto-certifications-web-ui` — see its own spec,
`docs/specs/category-management.md` in that app, for the UI side of this feature.
**Supersedes:** `frontend/docs/specs/11-certifications-journey.md` FR-6 ("categories
created implicitly, never predefined"), §7.1 `categories` table, §12.4 (hash-derived
color), and the `CategoryRepository` port in §6.5. Those sections should be read as
historical context only; this document is the current source of truth for the
category bounded concept.

## Problem & goal

Today a category is a bare `(id, path, normalized_path)` row, created **implicitly**
the first time a certification references a new path (`CategoryRepository.GetOrCreate`).
There is no create/update/delete surface, and no color is persisted — the timeline
derives a display color by hashing the category `id` into a fixed 8-color palette
(`TimelinePage`/`TimelineCanvas`, frontend).

The user runs this app single-tenant, as its own admin, and wants to manage
categories explicitly and give each one a real color that shows up wherever the
category is rendered (list, detail, timeline).

**Goal:** categories become an explicitly-managed resource — full CRUD via
`/api/v1/categories` — with a required, persisted, user-chosen hex color.
Certifications keep referencing a category by **path** on the wire (no breaking
change to `POST/PATCH /certifications`), but that path must now resolve to an
**existing** category; the backend no longer auto-creates one on the fly.

## Non-goals

- No authN/authZ. This backend already has none (no session/user concept
  anywhere in the codebase); "admin only" is satisfied by the app being
  single-tenant and not internet-exposed beyond the platform's own network.
  Do not add an auth layer as part of this change.
- No category hierarchy/merge/rename-cascade beyond what a path rename already
  gives for free (certifications reference the category by `id`, so renaming a
  category's path instantly updates every certification that shows it — no
  migration needed).
- No bulk import/export of categories.
- No soft-delete for categories (unlike certifications). A category delete is
  a hard delete, allowed only when unused — see BR-CAT-DEL1 below.
- No change to `GET /certifications` filtering-by-category-prefix behavior
  (§8.2 of Spec 11) beyond it continuing to work against the same columns.

## Domain model changes (`internal/certification/domain`)

1. New value object `CategoryColor` (new file `category_color.go` or added to
   `category.go`):
   - `NewCategoryColor(raw string) (CategoryColor, error)` — validates
     `^#[0-9A-Fa-f]{6}$` (after `strings.TrimSpace`), stores normalized
     **uppercase** hex (`#22D3EE`, not `#22d3ee`) so equality/comparison is
     stable. Invalid input → `SingleViolation("color", "format", "color must be
     a 6-digit hex value like #22D3EE")`.
   - `RehydrateCategoryColor(hex string) CategoryColor` — trusts persisted data,
     mirrors `RehydrateCategoryPath`.
   - `(c CategoryColor) String() string`.
2. `Category` struct gains `Color CategoryColor`.
3. `CategoryWithCount` unchanged in shape (embeds `Category`, adds `Count`) but
   now represents **every** category, not only in-use ones (see §"List" below).

## Ports (`internal/certification/app/port/out/category_repository.go`)

Replace the current `GetOrCreate` / `ListInUse` port with an explicit CRUD
surface:

```go
type CategoryRepository interface {
	Create(ctx context.Context, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error)
	Update(ctx context.Context, id string, path domain.CategoryPath, color domain.CategoryColor) (domain.Category, error)
	Delete(ctx context.Context, id string) error
	GetByPath(ctx context.Context, path domain.CategoryPath) (domain.Category, error)
	List(ctx context.Context) ([]domain.CategoryWithCount, error)
}
```

- `GetByPath` returns `domain.ErrNotFound` when no category has that normalized
  path — this is what a certification create/update now resolves against
  instead of auto-creating.
- `List` **left-joins** `certifications` (count `0` for categories with no
  living certification), ordered by `path` — it must return categories nobody
  has used yet, since that's exactly what the new "create category" form
  populates, and the certification form's dropdown must offer them too.
- `Create`/`Update` do **not** parse Postgres unique-violation errors. Following
  the existing convention (`createService.Execute` pre-checks
  `certs.ExistsByName` before insert), the **use case** layer pre-checks
  `GetByPath` for a colliding normalized path and returns a `ValidationError`
  before ever calling `Create`/`Update`. The small TOCTOU race is an accepted,
  precedented trade-off in this codebase (personal, single-writer app).
- `Delete` fails with a validation violation
  (`SingleViolation("category", "in_use", "category is used by N certification(s)")`)
  when at least one non-deleted certification still references the id, and
  `domain.ErrNotFound` when the id doesn't exist. (Both counted in the same
  query the repository already needs to run.)

Postgres adapter (`internal/certification/adapter/out/postgres/category_repo.go`):
rewrite around these five methods; drop `GetOrCreate`/`ListInUse`.

## Use cases (`internal/certification/app/usecase/`)

New files, mirroring the existing `create.go`/`update.go`/`delete.go` shape:

- `create_category.go` — `CreateCategoryService`: validate `CategoryPath` +
  `CategoryColor`, pre-check uniqueness via `GetByPath`, call `Create`.
- `update_category.go` — `UpdateCategoryService`: same validation for whichever
  fields are supplied (path and/or color — see DTO below), pre-check rename
  collisions against a *different* id, call `Update`.
- `delete_category.go` — `DeleteCategoryService`: call `Delete`, propagate its
  errors unchanged.

Existing files change:

- `list.go` (`ListCategoriesService`): call `categories.List` instead of
  `categories.ListInUse`.
- `create.go` / `update.go` (certification use cases): replace
  `s.categories.GetOrCreate(ctx, path)` with `s.categories.GetByPath(ctx, path)`.
  **Wrap** a `domain.ErrNotFound` result into
  `domain.SingleViolation("category", "exists", "category does not exist — create it first")`
  before returning — a certification create/update must never surface a bare
  404 for this, it stays a 422 on the certification endpoint, consistent with
  every other field-level rejection there.

New in-ports (`internal/certification/app/port/in/commands.go`):

```go
type CreateCategoryCommand struct {
	Path  string
	Color string
}
type CreateCategory interface {
	Execute(ctx context.Context, cmd CreateCategoryCommand) (domain.Category, error)
}

type UpdateCategoryCommand struct {
	Path  *string
	Color *string
}
type UpdateCategory interface {
	Execute(ctx context.Context, id string, cmd UpdateCategoryCommand) (domain.Category, error)
}

type DeleteCategory interface {
	Execute(ctx context.Context, id string) error
}
```

`UpdateCategoryCommand` uses plain optional pointers, not the `Patch[T]`
triple-state used for certifications — neither `path` nor `color` is a
clearable/nullable field, so "absent" unambiguously means "leave unchanged".

## Database design

New migration `0004_add_category_color` (append-only, following the existing
`000N_*.up.sql`/`.down.sql` pair convention):

```sql
-- 0004_add_category_color.up.sql
ALTER TABLE categories ADD COLUMN color text;

-- Backfill: deterministic assignment from the same 8-color palette the
-- frontend already uses, cycling by path order, so nothing visually jumps
-- for existing data once the frontend switches from hash-derived color to
-- this persisted one.
WITH ordered AS (
  SELECT id, row_number() OVER (ORDER BY path) - 1 AS rn FROM categories
),
palette(idx, hex) AS (
  VALUES (0,'#22D3EE'),(1,'#A78BFA'),(2,'#FBBF24'),(3,'#34D399'),
         (4,'#FB7185'),(5,'#38BDF8'),(6,'#A3E635'),(7,'#E879F9')
)
UPDATE categories c
SET color = p.hex
FROM ordered o JOIN palette p ON p.idx = (o.rn % 8)
WHERE c.id = o.id;

ALTER TABLE categories
    ALTER COLUMN color SET NOT NULL,
    ADD CONSTRAINT categories_color_hex CHECK (color ~ '^#[0-9A-F]{6}$');
```

```sql
-- 0004_add_category_color.down.sql
ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_color_hex;
ALTER TABLE categories DROP COLUMN IF EXISTS color;
```

`certification_repo.go` must add `k.color` to every `SELECT ... JOIN categories
k` (the create/update `RETURNING`-adjacent select, the get-by-id select, and
the list select — three sites today) and the row-scan struct in `mapping.go`
gains `CategoryColor string`; `mapping.go`'s `domain.Category{...}` construction
becomes
`domain.Category{ID: r.CategoryID, Path: domain.RehydrateCategoryPath(...), Color: domain.RehydrateCategoryColor(r.CategoryColor)}`.

## REST API (`api/openapi.yaml` + `internal/certification/adapter/in/http`)

New endpoints, added to the existing `/api/v1/categories` family:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/categories` | **Behavior change:** now returns every category (color + live-cert count, `0` for unused), not only in-use ones. |
| `POST` | `/api/v1/categories` | Create. Body: `{ "path": string, "color": string }`. `201` + the created category. |
| `PATCH` | `/api/v1/categories/{id}` | Update. Body: `{ "path"?: string, "color"?: string }`, at least one present. `200` + the updated category. |
| `DELETE` | `/api/v1/categories/{id}` | Delete. `204` on success. |

No `GET /categories/{id}` — the management UI works entirely off the list
response; adding a single-resource getter with no consumer is out of scope
(YAGNI).

`categoryDTO` (`dto.go`) gains `Color string \`json:"color"\`` — since this
struct is reused both standalone and embedded in `certificationDTO`, every
certification response also starts carrying its category's color at no extra
cost, which is exactly what the frontend timeline needs.

New request DTOs:

```go
type createCategoryRequest struct {
	Path  string `json:"path"`
	Color string `json:"color"`
}
type updateCategoryRequest struct {
	Path  *string `json:"path,omitempty"`
	Color *string `json:"color,omitempty"`
}
```

New handlers file `handlers_categories.go` (`handleCreateCategory`,
`handleUpdateCategory`, `handleDeleteCategory`), wired into `router.go`'s
`RouterDeps` (`CreateCategory`, `UpdateCategory`, `DeleteCategory` fields) and
`cmd/api/main.go`'s composition root, following the exact shape already used
for `Create`/`Update`/`Delete` on certifications.

`updateCategoryRequest` with both fields `nil` → `translateError` path via
`domain.SingleViolation("body", "required", "at least one of path or color must be provided")`,
returned by the handler before calling the use case.

No new entries needed in `errors.go`'s `translateError` — every failure mode
above (`ValidationError`, `ErrNotFound`) already maps correctly.

## Acceptance criteria

1. `POST /api/v1/categories {"path":"Cloud / AWS","color":"#22D3EE"}` → `201`,
   body `{id, path:"Cloud / AWS", color:"#22D3EE", count:0}`.
2. `POST` with a path whose normalized form already exists → `422
   validation_failed`, details include `{field:"path", rule:"unique"}`. No row
   created.
3. `POST` with `color:"blue"` → `422`, `{field:"color", rule:"format"}`.
4. `POST` with `path:""` → `422`, category-path violation (existing
   `NewCategoryPath` rule, field `category`/`required`).
5. `GET /api/v1/categories` includes a category with `count:0` created by (1)
   above, even though no certification references it yet.
6. `PATCH /api/v1/categories/{id} {"color":"#34D399"}` → `200`; `path`
   unchanged; `color` updated.
7. `PATCH {"path":"Cloud / GCP"}` → `200`; every certification that referenced
   this category id now shows `category.path = "Cloud / GCP"` on its next
   `GET`, with no data migration.
8. `PATCH` with a `path` colliding with a **different** existing category →
   `422`, `{field:"path", rule:"unique"}`. Patching a category to its own
   current path is not a collision (must not self-reject).
9. `PATCH` on an unknown id → `404 not_found`.
10. `DELETE /api/v1/categories/{id}` for a category with `count:0` → `204`; a
    following `GET /api/v1/categories` no longer lists it.
11. `DELETE` for a category referenced by ≥1 non-deleted certification → `422`,
    `{field:"category", rule:"in_use"}`; the row is not removed.
12. `DELETE` on an unknown id → `404 not_found`.
13. `POST /api/v1/certifications {"category":"Never Created Before", ...}` →
    `422`, `{field:"category", rule:"exists"}`. No row is created in
    `categories` (this is the behavior change from implicit to explicit
    creation — previously this same call would have silently created the
    category and succeeded with `201`).
14. `POST /api/v1/certifications {"category":"Cloud / AWS", ...}` where
    "Cloud / AWS" already exists → `201`; the response's `category.color`
    matches that category's persisted color.
15. `PATCH /api/v1/certifications/{id} {"category":"Also Never Created"}` →
    `422`, same `exists` violation as (13) — the update path resolves
    categories the same way create does.

## Testing plan (TDD)

- `internal/certification/domain`: table-driven tests for `NewCategoryColor`
  (valid lower/upper hex, missing `#`, wrong length, non-hex chars, whitespace
  trimming) in a new `category_color_test.go`, alongside the existing
  `values_test.go` pattern.
- `internal/certification/app/usecase`: extend `usecase_test.go` (or a new
  `category_usecase_test.go`) with the create/update/delete category services
  against `testsupport/fakes.go`'s fake repository — extend the fake to
  implement the new `CategoryRepository` interface. Cover AC 1–4, 6–13, 15
  above at the use-case level (no DB).
- `internal/certification/adapter/out/postgres/postgres_test.go`: extend with
  the new repository methods against the real Postgres test container/fixture
  already used there — cover the backfill migration (AC-5's shape: unused
  categories are listed) and the unique-path race-free-enough pre-check.
- `internal/certification/adapter/in/http/api_test.go` /
  `contract_test.go`: full-stack HTTP tests for the three new endpoints plus
  the changed `GET /categories` and the changed certification create/update
  behavior (AC 13, 15), mirroring the existing request/response assertion
  style in that file.
- Run with the project's existing commands: `make test` (`go test ./...`),
  `make vet`.

## Migration & rollout

- `MIGRATE_ON_START=true` (already set in `docker-compose.yml` for this
  service) applies `0004_...` automatically on next `docker compose up -d`.
  No manual step.
- Because the wire shape of `POST/PATCH /certifications` is unchanged
  (`category` stays a path string) except for the new *rejection* case (AC-13,
  15), this is a backward-incompatible **behavior** change but not a
  backward-incompatible **schema** change for existing consumers who always
  send an existing category path — which is the only way the frontend (after
  its own spec's changes) will ever call it.

## Open questions

None outstanding — the two decisions that would otherwise block this spec
(color input style: free hex; management scope: full CRUD) were confirmed by
the app owner before this document was written.
