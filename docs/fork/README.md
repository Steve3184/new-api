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

## Multi-generation Playground

The default frontend Playground now provides in-view tabs for chat, image
generation/editing, text-to-speech, and asynchronous 3D generation. The
classic frontend remains unchanged. Chat is the only feature enabled by
default, preserving the previous deployment behavior.

`PlaygroundSettings` is one additive JSON option with this shape:

```json
{
  "enabled_features": ["chat"],
  "models": {
    "chat": [],
    "image": [],
    "speech": [],
    "three_d": []
  },
  "speech_model_types": {}
}
```

An empty model list means all models available to that user and group are
allowed. A non-empty list is enforced by the backend, not only filtered in the
browser. Speech model types are `openai` (default) or `azure`. The public status
payload exposes the normalized configuration under `playground`.

All generation tabs use the same combined model/group picker as chat. The
frontend loads model availability for every usable group in parallel, builds a
union model list, and filters the group column for the selected model. A group
that does not provide the selected model is not shown; if a model change makes
the current group invalid, the first eligible group is derived immediately and
used by the request. The selection no longer relies on effect-driven state
correction when a generation tab mounts. The group column keeps short rows at
their natural height when only a few groups are eligible. Only the combined
picker is rendered, rather than separate model and group fields.

Session-authenticated Playground relay routes are additive equivalents of the
existing token-authenticated APIs:

| Route | Purpose |
| --- | --- |
| `POST /pg/chat/completions` | Chat |
| `POST /pg/images/generations` | Image generation |
| `POST /pg/images/edits` | Multipart image editing |
| `POST /pg/audio/speech` | Speech generation |
| `POST /pg/3d` | Submit a 3D task |
| `GET /pg/3d/:task_id` | Poll a user-owned 3D task |

Image controls use the OpenAI Images request shape and include 1K, 2K, and 4K
size presets. GPT Image and Seedream-compatible channels continue through the
existing image adaptors. Gemini image models, including Nano Banana aliases,
are converted to native `generateContent` image requests; uploaded edit images
become Gemini `inlineData`, and Gemini image parts are converted back to the
OpenAI Images response shape. `gpt-image-2` is included in the OpenAI model
catalog. VolcEngine image edits convert the first multipart upload to the data
URI accepted by the Seedream generations endpoint, and the channel catalog
includes Seedream 4.0 plus generic Seedream 5.0 aliases.

The image and 3D desktop layouts use the full Playground width: their fixed
control columns align with the left edge and the remaining width belongs to the
result workspace. Every generated image also has an edit action that converts
that workspace result into the multipart source file, switches to edit mode,
and returns the user to the controls.

Speech always sends the required `speed` float (`1.0` baseline). Azure-typed
models additionally expose optional `volume` (`1.0` baseline) and integer
`pitch` in Hz (`0` baseline); those fields are omitted for OpenAI-typed models.
The Azure voice selector vendors the 322 names from
`s3aidocs/docs/.vitepress/dist/azure-tts-voice-list.txt` as 322 distinct,
searchable combobox options. Volume and pitch each have an explicit opt-in
switch, so their baseline values are not sent unless the user enables that
parameter. The speech layout centers a flexible upper
editor where the text area consumes the remaining desktop height and the
parameter column keeps its natural width. The upper editor scrolls when space
is constrained. Its compact audio workspace has a fixed reserved height at the
bottom on both desktop and mobile instead of becoming a second desktop column.

The 3D tab supports text/image input, Meshy art styles, draft-to-texture source
task IDs, progress polling, GLB download, and a lazily loaded Three.js viewer.
It always confirms the locally persisted task state before mounting the viewer,
which avoids loading the content proxy while an immediately completed upstream
response is still being inserted locally. Transient task lookup errors are
retried, and GLB/GLTF load failures now produce an explicit UI state instead of
a blank canvas. On small screens, generation forms and result workspaces use a
single vertical scroll area; desktop keeps the split workspace.

For session-authenticated `/pg` submissions, distributor parsing preserves the
requested `group` alongside `model` for JSON and multipart bodies. Channel
selection therefore uses the group shown in the combined picker rather than
falling back to the user's default group.

Dynamic billing expressions add `req`, fixed at `1,000,000` in the v1
expression environment. Its coefficient is therefore a per-request USD price
while preserving the existing `$ / 1M` quota conversion. The additive
`image_resolution()` helper normalizes explicit `1K`/`2K`/`4K` quality values
and dimension strings such as `2048x2048` into stable resolution tiers. The
default frontend request-rule editor exposes this as an image-size condition
and includes a per-request 1K/2K/4K preset with editable multipliers. Non-JSON
image edits also freeze a normalized request body for pre-consume and
settlement, so multipart `size` values participate in the same pricing rule.

Files:

- `setting/playground_setting/playground_setting.go`
- `setting/playground_setting/playground_setting_test.go`
- `model/option.go`
- `controller/misc.go`
- `controller/playground.go`
- `controller/relay.go`
- `router/relay-router.go`
- `middleware/distributor.go`
- `middleware/distributor_playground_test.go`
- `relay/constant/relay_mode.go`
- `relay/relay_task.go`
- `relay/helper/valid_request.go`
- `relay/channel/openai/constant.go`
- `relay/channel/gemini/adaptor.go`
- `relay/channel/gemini/relay-gemini.go`
- `relay/channel/gemini/image_generation_test.go`
- `relay/channel/volcengine/adaptor.go`
- `relay/channel/volcengine/constants.go`
- `relay/channel/volcengine/image_edit_test.go`
- `dto/audio.go`
- `pkg/billingexpr/compile.go`
- `pkg/billingexpr/run.go`
- `pkg/billingexpr/expr.md`
- `pkg/billingexpr/billingexpr_test.go`
- `web/default/package.json`
- `web/bun.lock`
- `web/default/src/components/ui/combobox-input.tsx`
- `web/default/src/features/playground/`
- `web/default/src/features/pricing/lib/billing-expr.ts`
- `web/default/src/features/pricing/lib/dynamic-price.ts`
- `web/default/src/features/pricing/lib/tier-expr.ts`
- `web/default/src/features/system-settings/models/playground-settings-card.tsx`
- `web/default/src/features/system-settings/models/index.tsx`
- `web/default/src/features/system-settings/models/section-registry.tsx`
- `web/default/src/features/system-settings/models/tiered-pricing-editor.tsx`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`
- `web/default/src/features/auth/types.ts`
- `web/default/src/i18n/static-keys.ts`
- `web/default/src/i18n/locales/*.json`

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

## API key coding-tool setup (2026-07-13)

The API key cell has a permanently visible Terminal action beside the existing
copy button. It resolves the full key on demand and opens a setup dialog for
Codex, OpenCode, and Claude Code; the action is intentionally not hidden in the
row overflow menu.

The dialog derives its base URL from `ServerAddress` (falling back to the current
origin), uses the selected key's group, and loads models through
`/api/user/models?group=<group>`. The model selector is placed above the tool
tabs and does not receive initial focus when the dialog opens. Selecting a model
updates all generated configurations:

- Codex uses a custom `newapi` Responses provider, file-based `auth.json`,
  sandbox network access, and the selected model. The review model is
  `codex-auto-review` when the selected model ID contains `gpt-`; otherwise it
  matches the selected model.
- OpenCode dynamically generates its `models` object from the selected group's
  available models. Each entry uses the model ID as its display name and omits
  guessed context/output limits.
- Claude Code writes the selected model and includes
  `CLAUDE_CODE_ATTRIBUTION_HEADER=0`,
  `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`, the API key, and an
  Anthropic-format base URL ending in `/` without `/v1`.

Every path and configuration block has a copy action. CodeMirror content uses a
dialog-specific horizontal inset so configuration text does not touch the modal
edge.

`GroupDefaultModel` is an additive JSON option mapping group names to the model
initially selected in the dialog. The group-pricing settings page exposes a
portal-based searchable selector for each group; each selector queries that
group's available models and still permits a custom model ID. Review models are
not persisted separately.

`AutoGroupDescription` is an optional string option shown in group settings only
when `AutoGroups` is non-empty. `/api/user/self/groups` exposes the virtual
`auto` group only when the current user has at least one usable automatic group,
and uses this description when configured.

Files:

- `setting/auto_group.go`
- `setting/ratio_setting/group_model.go`
- `setting/ratio_setting/group_model_test.go`
- `model/option.go`
- `controller/group.go`
- `controller/option.go`
- `web/default/src/features/keys/components/api-keys-cells.tsx`
- `web/default/src/features/keys/components/api-keys-dialogs.tsx`
- `web/default/src/features/keys/components/dialogs/api-key-usage-dialog.tsx`
- `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- `web/default/src/features/keys/components/dialogs/cc-switch-dialog.tsx`
- `web/default/src/features/keys/types.ts`
- `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`
- `web/default/src/features/system-settings/models/group-coding-model-editor.tsx`
- `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- `web/default/src/features/system-settings/models/ratio-settings-card.tsx`
- `web/default/src/features/system-settings/models/index.tsx`
- `web/default/src/features/system-settings/billing/index.tsx`
- `web/default/src/features/system-settings/billing/section-registry.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/lib/api.ts`
- `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## Meshy2API native 3D provider

Channel type `59` adds native Meshy2API support. It uses a dedicated task
adapter and the `/v1/3d` protocol end to end; it does not route 3D requests
through the OpenAI/Sora video API.

Configure a channel with:

| Field | Value |
| --- | --- |
| Type | `Meshy2API` (`59`) |
| Base URL | Meshy2API service origin, without `/v1` |
| Key | API key configured in Meshy2API |
| Models | One or more model names from the list below |

The channel test action is intentionally disabled because a real 3D test would
create a billable asynchronous generation task. Use the API examples below for
an explicit end-to-end test.

### API contract

Create a task:

```http
POST /v1/3d
Authorization: Bearer sk-new-api-key
Content-Type: application/json
```

Poll a task and download its GLB result:

```http
GET /v1/3d/{task_id}
GET /v1/3d/{task_id}/content
```

Supported model names:

| Base model | Full pipeline | Draft only | Texture an existing draft |
| --- | --- | --- | --- |
| Meshy 6 | `meshy-6` | `meshy-6-draft` | `meshy-6-texture` |
| Meshy 5.3 | `meshy-5.3` | `meshy-5.3-draft` | `meshy-5.3-texture` |
| Meshy 5.1 | `meshy-5.1` | `meshy-5.1-draft` | `meshy-5.1-texture` |
| Meshy 5 | `meshy-5` | `meshy-5-draft` | `meshy-5-texture` |
| Meshy 4 | `meshy-4` | `meshy-4-draft` | `meshy-4-texture` |

`art_style` accepts `realistic`, `cartoon`, `sculpture`, or `pbr`.
`input_reference` accepts a base64 data URL or bare base64 image. HTTP image
URLs are rejected so the gateway never fetches a caller-controlled URL.

Text-to-3D example:

```json
{
  "model": "meshy-6",
  "prompt": "a medieval wooden treasure chest with iron bands",
  "metadata": {
    "art_style": "cartoon"
  }
}
```

Image-to-3D example:

```json
{
  "model": "meshy-6-draft",
  "input_reference": "data:image/png;base64,iVBORw0KGgo...",
  "metadata": {
    "art_style": "realistic"
  }
}
```

Create texture for an existing draft:

```json
{
  "model": "meshy-6-texture",
  "source_task_id": "task_public_draft_id",
  "prompt": "weathered oak with dark iron bands",
  "metadata": {
    "art_style": "pbr"
  }
}
```

`source_task_id` is the public `id` returned by new-api for a completed draft.
Clients must not pass a Meshy or Meshy2API internal ID. new-api verifies that
the source belongs to the caller, is complete, was created by the same
Meshy2API channel, and uses the same base model. It then replaces the public ID
with the stored upstream task ID before forwarding the request.

Submission response:

```json
{
  "id": "task_a_public_new_api_id",
  "object": "3d",
  "model": "meshy-6-draft",
  "status": "queued",
  "progress": 0,
  "created_at": 1783495163
}
```

Completed response:

```json
{
  "id": "task_a_public_new_api_id",
  "object": "3d",
  "model": "meshy-6-draft",
  "status": "completed",
  "progress": 100,
  "created_at": 1783495163,
  "completed_at": 1783495245,
  "data": {
    "format": "glb",
    "url": "https://new-api.example.com/v1/3d/task_a_public_new_api_id/content"
  }
}
```

The public response never exposes the upstream task ID or upstream artifact
URL. The content endpoint authenticates the caller, checks task ownership, and
streams the GLB from the original Meshy2API channel.

### Billing

3D tasks use the existing asynchronous per-call billing lifecycle. Configure a
fixed model price for every enabled full, draft, and texture model name. A
failed upstream task follows the normal asynchronous task refund path. Full
pipeline prices should include both draft and automatic texture generation.

### Compatibility boundary

- Meshy2API native support is available only through `/v1/3d`.
- No `/v1/videos` compatibility route is registered by the Meshy2API service.
- Existing Sora/video channels and their `/v1/videos` behavior are unchanged.
- Existing database schemas are reused; task ownership, upstream IDs, API keys,
  billing context, and result URLs remain in the existing task record.
- All database behavior remains compatible with SQLite, MySQL, and PostgreSQL.

### Files

- `common/endpoint_defaults.go`
- `common/endpoint_type.go`
- `constant/channel.go`
- `constant/endpoint_type.go`
- `controller/channel-test.go`
- `controller/model.go`
- `controller/relay.go`
- `controller/swag_three_d.go`
- `controller/three_d_proxy.go`
- `dto/three_d.go`
- `middleware/distributor.go`
- `model/task.go`
- `relay/channel/adapter.go`
- `relay/channel/task/meshy/adaptor.go`
- `relay/channel/task/meshy/adaptor_test.go`
- `relay/channel/task/meshy/constants.go`
- `relay/channel/task/taskcommon/helpers.go`
- `relay/common/relay_info.go`
- `relay/common/relay_utils.go`
- `relay/constant/relay_mode.go`
- `relay/relay_adaptor.go`
- `relay/relay_task.go`
- `relay/relay_task_three_d_test.go`
- `router/video-router.go`
- `web/default/scripts/sync-i18n.mjs`
- `web/default/src/features/channels/constants.ts`
- `web/default/src/features/channels/lib/channel-type-config.ts`
- `web/default/src/features/channels/lib/channel-utils.ts`
- `web/default/src/features/models/constants.ts`
- `web/default/src/features/pricing/components/model-details-api.tsx`
- `web/default/src/features/pricing/constants.ts`
- `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

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
13. On the API key page, verify the Terminal action remains visible beside copy,
    each tool config uses the selected key/group/model, and all copy actions
    include the full resolved key and normalized site URL.
14. In group pricing, verify model selectors are not clipped by the table,
    request only group-available models, and the optional auto-group description
    is returned only when automatic grouping is usable.
