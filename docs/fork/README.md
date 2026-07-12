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
link flag. Internal URLs must start with `/`; external URLs must use HTTP or
HTTPS. Icons are selected from a bounded Lucide set to avoid bundling the full
icon library.

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
8. Test wallet Markdown, internal/external custom tabs, vendor CRUD, and Model
   Square pricing with and without a selected group.
