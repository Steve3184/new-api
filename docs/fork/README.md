# Custom Fork Change Manifest

This directory documents the additive customizations maintained on top of
QuantumNous/new-api. The classic frontend is intentionally unchanged; all UI
work is under `web/default`.

## Repository layout and upstream workflow

Use `origin` for this fork and keep the official repository as `upstream`:

```bash
git remote add upstream https://github.com/QuantumNous/new-api.git
git fetch upstream
git switch -c upstream-sync/YYYY-MM-DD
git merge upstream/main
```

The current repository already uses
`https://github.com/Steve3184/new-api` as `origin`. Add `upstream` only once;
after that, regular synchronization starts with `git fetch upstream`.

Resolve conflicts on the temporary `upstream-sync/*` branch, run the checks in
this document, then merge that branch into the fork's main branch. Do not keep
custom changes as a copied second source tree; keeping one Git history makes
upstream conflict resolution auditable.

## Compatibility rules

- Existing option names and response fields are preserved.
- New settings are stored as additive rows in the existing `options` table; no
  database migration or database-specific SQL is required.
- SQLite, MySQL, and PostgreSQL behavior is unchanged.
- The default captcha provider remains Turnstile, so existing deployments keep
  their previous behavior until Cap is explicitly configured and selected.
- No files under `web/classic` are modified.

## Cap captcha integration

Cap uses a self-hosted Cap Standalone instance. Configure two Cap site keys when
login/registration and check-in use different PoW difficulties. The public
widget endpoint is `<CapServerURL>/<siteKey>/`; token verification uses
`<CapServerURL>/<siteKey>/siteverify`.

New options:

| Option | Purpose |
| --- | --- |
| `CaptchaType` | Active provider: `turnstile` or `cap` |
| `CapEnabled` | Enables Cap |
| `CapServerURL` | Public Cap Standalone base URL |
| `CapAdminAPIKey` | Cap API key used to update site-key difficulty |
| `CapSiteKey` / `CapSecretKey` | Login, registration, and recovery key pair |
| `CapCheckinSiteKey` / `CapCheckinSecretKey` | Daily check-in key pair |
| `LoginCaptchaDifficulty` | Login/registration difficulty, range 1-8 |
| `CheckinCaptchaDifficulty` | Check-in difficulty, range 1-8 |
| `ForceCheckinCaptcha` | Requires a fresh captcha for every check-in |

Difficulty is not sent as a browser-controlled widget attribute. When a Cap
setting changes, the backend updates Cap Standalone through
`PUT /server/keys/:siteKey/config`. If the two difficulties differ, the two
flows must use different Cap site keys.

`GetOptions` returns `"***"` for sensitive fields (`CapSecretKey`,
`CapAdminAPIKey`, `CapCheckinSecretKey`, `TurnstileSecretKey`) that are already
set, rather than omitting them. This lets the settings form distinguish
"already configured" from "never set" so it can skip the required-field
validation and avoid blocking saves when only a boolean toggle like `CapEnabled`
changes. The frontend renders these fields empty with an explanatory placeholder
via a local `SensitiveInput` wrapper; `UpdateOption` rejects the `"***"`
sentinel if it somehow arrives, preventing accidental overwrites.

The widget dependency is loaded from the pinned CDN release
`cap-widget@0.1.50`; review and update that version deliberately during
upstream syncs instead of following the moving `latest` tag.

Files:

- `common/constants.go`
- `model/option.go`
- `controller/misc.go`
- `controller/option.go`
- `middleware/cap-check.go`
- `middleware/cap-check_test.go`
- `middleware/captcha-check.go`
- `middleware/turnstile-check.go`
- `router/api-router.go`
- `service/cap.go`
- `service/cap_test.go`
- `web/default/src/components/cap.tsx`
- `web/default/src/features/auth/api.ts`
- `web/default/src/features/auth/types.ts`
- `web/default/src/features/auth/hooks/use-captcha.ts`
- `web/default/src/features/auth/hooks/use-email-verification.ts`
- `web/default/src/features/auth/sign-in/components/user-auth-form.tsx`
- `web/default/src/features/auth/sign-up/components/sign-up-form.tsx`
- `web/default/src/features/auth/forgot-password/components/forgot-password-form.tsx`
- `web/default/src/features/profile/api.ts`
- `web/default/src/features/profile/components/checkin-calendar-card.tsx`
- `web/default/src/features/profile/index.tsx`
- `web/default/src/features/system-settings/auth/bot-protection-section.tsx`
- `web/default/src/features/system-settings/auth/index.tsx`
- `web/default/src/features/system-settings/auth/section-registry.tsx`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`
- `web/default/src/features/system-settings/types.ts`

## Payment announcement

`PaymentAnnouncement` is an optional Markdown announcement configured with the
payment gateway settings and rendered below the available payment methods on
the wallet page. Rendering uses the existing sanitized Markdown component.

Files:

- `common/constants.go`
- `model/option.go`
- `controller/topup.go`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/system-settings/billing/index.tsx`
- `web/default/src/features/system-settings/billing/section-registry.tsx`
- `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`
- `web/default/src/features/wallet/types.ts`
- `web/default/src/features/wallet/components/recharge-form-card.tsx`

## Custom console tabs

`CustomTabs` stores a JSON array of up to 50 links. Each entry has an ID, label,
URL, icon, category (`chat`, `general`, `personal`, or `admin`), and an external
link flag. URLs may be internal paths starting with `/` or absolute HTTP/HTTPS
URLs. URL validation is independent of the external-link flag: the flag controls
whether the link opens in a new tab, not whether an absolute URL is accepted.
Icons are selected from a bounded Lucide set to avoid bundling the full icon
library.

When "Open in new tab" is unchecked (`external: false`), clicking the sidebar
entry renders the target URL inside a full-height iframe within the app shell.
The sidebar link points to `/custom-tab/{id}` rather than the raw URL, and the
dedicated route looks up the tab from the status payload and mounts the iframe.
Admin-category tabs are hidden from non-admin users by the existing group-level
role filter in `use-sidebar-view.ts`.

Files:

- `common/constants.go`
- `model/option.go`
- `controller/misc.go`
- `controller/option.go`
- `web/default/src/features/auth/types.ts`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/system-settings/content/index.tsx`
- `web/default/src/features/system-settings/content/section-registry.tsx`
- `web/default/src/features/system-settings/content/custom-tabs-section.tsx`
- `web/default/src/lib/custom-tabs.ts`
- `web/default/src/lib/custom-tabs.test.ts`
- `web/default/src/hooks/use-sidebar-data.ts`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`
- `web/default/src/components/layout/types.ts`
- `web/default/src/components/layout/components/nav-group.tsx`
- `web/default/src/routes/_authenticated/custom-tab/$tabId/index.tsx`

## Vendor management fix

The Models page's **Manage Vendors** action now opens a vendor list rather than
the create form. The dialog supports creating, editing, and deleting vendors
and refreshes model/vendor queries after mutations.

Files:

- `web/default/src/features/models/components/models-primary-buttons.tsx`
- `web/default/src/features/models/components/models-provider.tsx`
- `web/default/src/features/models/components/models-dialogs.tsx`
- `web/default/src/features/models/components/dialogs/vendors-manage-dialog.tsx`

## Model Square group pricing fix

Without a group filter, model summaries show the base price rather than the
lowest enabled-group price. With a group selected, all request, token, cache,
and dynamic-pricing summaries use that group's ratio.

File:

- `web/default/src/features/pricing/lib/model-helpers.ts`

## Authentication form separator position fix

The separator divider on login and registration pages now appears after OAuth
provider buttons instead of before them, improving visual hierarchy. Login page
order is now: Passkey → OAuth buttons → separator → username/password form.
Registration page order is now: OAuth buttons → separator → username/password/
email form.

File:

- `web/default/src/features/auth/components/oauth-providers.tsx`

## Translations

All new default-frontend text is present in English, Simplified Chinese,
Traditional Chinese, French, Japanese, Russian, and Vietnamese.

Files:

- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/zh-TW.json`
- `web/default/src/i18n/locales/fr.json`
- `web/default/src/i18n/locales/ja.json`
- `web/default/src/i18n/locales/ru.json`
- `web/default/src/i18n/locales/vi.json`

## Playground starter prompts

Each entry in `starterPrompts` now carries a distinct `prompt` field with the
actual English text sent to the model. Previously the button label (the short
translated string) was sent directly, so clicking "Analyze data" sent the words
"Analyze data" rather than a useful prompt.

File:

- `web/default/src/features/playground/components/chat/playground-empty-state.tsx`

## Default frontend maintenance batch (2026-07-13)

This batch fixes several default-frontend regressions and completes previously
stubbed Playground attachment actions. It intentionally does not change the
classic frontend.

### Table row-height isolation

The shared table body no longer forces every row to `h-15`. The fixed height is
applied only by the usage-log table, which is the surface that needs room for
the token timing metrics. This prevents unrelated user, token, channel, and
settings tables from becoming taller when usage-log timing fields are added.

Files:

- `web/default/src/components/ui/table.tsx`
- `web/default/src/features/usage-logs/components/usage-logs-table.tsx`

### Playground editor stability and attachments

The history-message CodeMirror editor now keeps key handlers in a ref instead
of recreating the editor extension whenever the parent renders. This preserves
the selection and IME composition state while editing an existing message.

Playground attachment actions now support regular files, images, camera capture,
and browser screen capture. Selected files are previewed, converted from blob
URLs to data URLs before submission, retained when conversion/submission fails,
and rendered on the submitted user message. Images are sent as `image_url`
content parts; other files use file content parts with `filename` and
`file_data`. Attachment-only messages are supported. The input limits each
message to 8 files with a 10 MiB per-file limit.

Files:

- `web/default/src/components/ai-elements/code-block.tsx`
- `web/default/src/components/ai-elements/prompt-input.tsx`
- `web/default/src/features/playground/components/input/playground-input-controls.tsx`
- `web/default/src/features/playground/components/input/playground-input-tools.tsx`
- `web/default/src/features/playground/components/input/playground-input.tsx`
- `web/default/src/features/playground/components/message/playground-message-content.tsx`
- `web/default/src/features/playground/hooks/use-playground-conversation.ts`
- `web/default/src/features/playground/lib/input/input-control-utils.ts`
- `web/default/src/features/playground/lib/input/input-control-utils.test.ts`
- `web/default/src/features/playground/lib/input/input-tool-utils.ts`
- `web/default/src/features/playground/lib/message/conversation-message-utils.ts`
- `web/default/src/features/playground/lib/message/message-content-utils.ts`
- `web/default/src/features/playground/lib/message/message-utils.ts`
- `web/default/src/features/playground/lib/message/message-utils.test.ts`
- `web/default/src/features/playground/types.ts`

### System notice popup

Two additive options control whether the existing HTML/Markdown system notice
is displayed as a dialog:

| Option | Purpose |
| --- | --- |
| `NoticePopupEnabled` | Shows the notice whenever the home route is opened |
| `NoticePopupOnDashboardEnabled` | Also shows it when the overview dashboard route is opened |

The dashboard option is effective only when the main popup option is enabled.
The dialog uses the existing sanitized rich-content renderer, has a top-right X
close control, and provides a **Close Today** action. Close-today state is stored
in the existing `notification-storage` local-storage entry and suppresses both
placements until the browser's local date changes. Ordinary X dismissal applies
only to the current mounted dialog, so reopening the configured route can show
the notice again.

New option rows use the existing option table and public status payload; no
schema migration is required.

Files:

- `common/constants.go`
- `model/option.go`
- `model/option_notice_popup_test.go`
- `controller/misc.go`
- `web/default/src/components/notice-popup.tsx`
- `web/default/src/features/auth/types.ts`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`
- `web/default/src/features/system-settings/maintenance/notice-section.tsx`
- `web/default/src/features/system-settings/site/index.tsx`
- `web/default/src/features/system-settings/site/section-registry.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/routes/index.tsx`
- `web/default/src/routes/_authenticated/dashboard/$section.tsx`

### User-table search responsiveness

The user table's username/name/email filter now waits 350 ms after typing before
committing the route and server-side search state. The input itself remains
immediate, avoiding a query and table rerender for every keystroke.

File:

- `web/default/src/features/users/components/users-table.tsx`

### First-token timing audit (no source change)

The current stream scanner records `FirstResponseTime` on the first non-empty
SSE `data:` frame that is not `[DONE]`. It does not wait for visible assistant
text. Consequently, a reasoning frame counts toward first-token time, and
Responses-style lifecycle frames such as `response.created` may make GPT model
TTFT appear lower than the time to the first reasoning or output token. The log
field `other.frt` and performance TTFT aggregation both derive from this same
timestamp. This behavior was audited but intentionally not changed in this
batch; review `relay/helper/stream_scanner.go` and provider-specific streaming
parsers before changing the metric definition during a future upstream sync.

### Local runtime override (not part of source sync)

The development SQLite database currently has `CapEnabled=false` to disable the
login captcha temporarily. This is deployment data in `one-api.db`, is not a Git
change, and must not be expected to transfer through an upstream merge. The
separate `ForceCheckinCaptcha` option remains enabled in that local database.

### Verification for this batch

- `go test ./common ./model ./controller`
- `bun run i18n:sync`
- `bun run typecheck`
- `bun run build`
- `git diff --check`
- Browser checks for custom absolute URLs, Playground editing/IME and uploads,
  notice popup placement/close-today behavior, table heights, and user search
  responsiveness

## Upstream sync checklist

1. Fetch and merge `upstream/main` on a temporary sync branch.
2. Review conflicts first in `model/option.go`, `controller/option.go`,
   `controller/misc.go`, `router/api-router.go`, the auth forms, system-settings
   registries, and `use-sidebar-data.ts`.
3. Confirm that no existing option or status field was removed or renamed.
4. Run `gofmt` on changed Go files.
5. Run `go test ./common ./model ./middleware ./service ./controller ./router`.
6. From `web/default`, run `bun run i18n:sync`, `bun run typecheck`, targeted
   lint for changed files, and `bun run build`.
7. Test Turnstile and Cap separately, including token reuse rejection and a
   forced check-in after an earlier login captcha.
8. Test wallet Markdown, internal/absolute custom-tab URLs with both open modes,
   vendor CRUD, and Model Square pricing with and without a selected group.
9. Test Playground history editing with an IME and verify file, image, camera,
   screen-capture, and attachment-only submissions.
10. Verify that only usage-log rows retain the fixed height and that user search
    commits after the debounce without delaying visible typing.
11. Test notice popup behavior on `/` and `/dashboard/overview`, including X
    dismissal, **Close Today** persistence, empty notices, and both option
    combinations.
12. If upstream changes stream event parsing, re-audit TTFT so lifecycle events
    are not accidentally treated as model tokens without an explicit metric
    decision.
