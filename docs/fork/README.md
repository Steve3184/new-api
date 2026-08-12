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
- New system settings are stored as additive rows in the existing `options`
  table. Subscription and redemption additions use the existing cross-database
  GORM migration path.
- SQLite, MySQL, and PostgreSQL behavior is unchanged.
- The default captcha provider remains Turnstile, so existing deployments keep
  their previous behavior until Cap is explicitly configured and selected.
- hCaptcha is available as an additive provider and is disabled by default.
- Check-in balance gating is disabled when `checkin_setting.min_user_quota` is
  `0`; when set, the user balance must be strictly greater than the configured
  NewAPI quota value.
- No files under `web/classic` are modified.

## OAuth binding and custom provider flow

The admin user-binding dialog sends canonical binding types (`github`,
`discord`, `oidc`, `wechat`, `telegram`, and `linuxdo`) instead of database
field names. The backend also accepts the previous field-name aliases for
compatibility, so existing clients can still clear bindings.

Custom OAuth providers use the same state-protected `/oauth/{slug}` callback
for login and popup binding. The frontend creates an OAuth flow, redirects to
the configured authorization endpoint, and sends the callback to
`/api/oauth/{slug}`. The backend exchanges the code at the configured token
endpoint, then either fetches the configured profile endpoint with the returned
Bearer token or, when that endpoint is empty, reads claims from the returned
OIDC `id_token` after verifying its issuer, audience, expiry, and signature
against the configured discovery document's JWKS. This supports providers whose
token and profile endpoints are separate as well as Telegram's OIDC flow, which
returns profile claims in the ID token and has no separate UserInfo endpoint.

The custom OAuth preset list includes Telegram with these defaults:
`https://oauth.telegram.org/auth`, `https://oauth.telegram.org/token`, scopes
`openid profile`, and claims `sub`, `preferred_username`, and `name`.

Files:

- `model/user.go`
- `model/user_binding_test.go`
- `oauth/generic.go`
- `oauth/generic_test.go`
- `model/custom_oauth_provider.go`
- `model/custom_oauth_provider_test.go`
- `controller/custom_oauth.go`
- `web/src/features/users/components/dialogs/user-binding-dialog.tsx`
- `web/src/features/system-settings/auth/custom-oauth/types.ts`
- `web/src/features/system-settings/auth/custom-oauth/components/preset-selector.tsx`
- `web/src/features/system-settings/auth/custom-oauth/components/provider-form-dialog.tsx`

## Per-group retry and channel recovery controls

`GroupRetryTimes` is an additive JSON option that overrides the global
`RetryTimes` value for individual groups. Values are integers from `0` through
`10`; an omitted group inherits the global setting, while an explicit `0`
disables retries for that group. The override is applied to synchronous relays,
asynchronous task submissions, channel-priority fallback, and each concrete
group selected by an `auto` token. The default frontend exposes the override in
the group-pricing table and in its JSON editor.

The scheduled channel-test mode `passive_recovery` only selects channels whose
status is `ChannelStatusAutoDisabled`. Manually disabled and currently enabled
channels are excluded, and failures during this mode cannot disable additional
channels. Successful probes re-enable an auto-disabled channel only when the
global automatic-enable option is active.

Each channel already has an **Auto Ban** switch in the advanced settings. Setting
`auto_ban` to `0` prevents automatic disabling from both real relay failures and
scheduled full-test failures; manual channel state changes remain available.

Files:

- `setting/ratio_setting/group_retry.go`
- `setting/ratio_setting/group_retry_test.go`
- `model/option.go`
- `controller/option.go`
- `controller/relay.go`
- `controller/relay_retry_test.go`
- `service/channel_select.go`
- `controller/channel-test.go`
- `controller/channel_test_internal_test.go`
- `web/default/src/features/system-settings/billing/index.tsx`
- `web/default/src/features/system-settings/billing/section-registry.tsx`
- `web/default/src/features/system-settings/models/group-ratio-form.tsx`
- `web/default/src/features/system-settings/models/group-ratio-visual-editor.tsx`
- `web/default/src/features/system-settings/models/ratio-settings-card.tsx`
- `web/default/src/features/system-settings/types.ts`
- `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## Group status page and flexible active probes

The default frontend exposes a **Status Check** sidebar entry at `/status`.
Each configured group is rendered as a card with 24-hour availability bars,
average latency, and upstream cache hit rate. Clicking a card opens a
right-side detail panel with hourly average-latency and cache hit-rate
line charts. Each chart includes a left-side Y axis with `ms` or percentage
tick labels and an hourly X axis. The chart bundle is loaded only when the
detail panel is opened. The page refreshes `GET /api/status-check` every 30
seconds. Availability lines use a fixed 3-pixel width and 4-pixel gap, while
their count adapts to the card's available width. The card grid uses an
explicit single-column track on mobile, and its contents can shrink without
widening the page.
Synchronous requests contribute to request, availability, and cache statistics
but do not produce a first-token timing sample. Average latency includes all
recorded requests.

`StatusCheckGroups` is an additive JSON-array option. Configure it under
**System Settings → Console Content → Status Check**. An empty array displays
all active groups, while a non-empty array limits the page to the selected
groups. Removed or inactive groups are ignored at read time.

`StatusCheckCacheExcludedModels` is an additive JSON-array option in the same
settings section. Requests from listed model names still contribute to
availability and latency, but their cache samples and hits are omitted from the
current and hourly cache hit-rate calculations. This lets administrators
exclude models or providers that do not support cache reporting.

`StatusCheckFlexibleMode` is an additive JSON-object option in the same
settings section. Its default is `{ "groups": {} }`, which creates no active
probes:

```json
{
  "groups": {
    "default": {
      "enabled": true,
      "idle_minutes": 15,
      "max_consecutive_probes": 40
    },
    "vip": {
      "enabled": false,
      "idle_minutes": 5,
      "max_consecutive_probes": 20
    }
  }
}
```

Each key is a separately configured group. Flexible probing requires a
non-empty explicit `StatusCheckGroups` list, and only groups that appear in
both that list and the JSON object with `enabled: true` are eligible. The
automatic `auto` group is never eligible. For every enabled group, the
scheduled task waits until that group has had no normal relay request for its
own `idle_minutes`, then tests one enabled channel in that group. A probe is a
no-charge channel test: its success result and end-to-end latency contribute to
the group's availability and average-latency data, while all cache counters and
cache-token fields remain zero. It never enables, disables, or otherwise
changes channel state.

The first probe waits for a full idle period after a group is configured. Each
subsequent automatic probe increments that group's consecutive-probe count. A
normal relay request resets the streak; once the count reaches
that group's `max_consecutive_probes`, automatic tests pause until normal
traffic resumes. The values are validated as `1..1440` idle minutes and
`1..1000` consecutive probes per group. The scheduler checks eligibility once
per minute so groups with different idle periods remain independent. Passive
activity is shared through Redis when available and uses a rate-limited
database fallback otherwise, so the schedule is safe across multiple master
nodes.

The status entry is part of the existing `SidebarModulesAdmin` and per-user
`sidebar_modules` console section under the additive `status` key. Existing
configurations default it to visible, while administrators and eligible users
can hide it with the normal sidebar module controls.

The `perf_metrics` table retains the additive request-level `cache_hit_count`
and `cache_sample_count` columns for compatibility and gains additive
`cached_tokens` and `input_tokens` columns through the existing GORM migration
path. The displayed cache hit rate is token-weighted:
`SUM(cached_tokens) / SUM(input_tokens) * 100`, after applying the configured
model exclusion list. Requests with valid input usage but no cached tokens add
zero to the numerator and their input tokens to the denominator. Missing or
non-positive input usage is omitted. Negative cached values are normalized to
zero. When cached tokens exceed the reported input tokens, cached tokens are
added to the denominator as a fallback; the final aggregate is still bounded
to the `0%` to `100%` range as a second defense against invalid historical
data.

`ChannelAutoStatusEmailEnabled` is a backward-compatible routing reliability
option that defaults to `true`. Disabling it suppresses email to the root
administrator when a channel is automatically disabled or re-enabled without
changing the automatic status transition or non-email notification channels.

The Playground generation configuration now appears under **Console Content**
instead of **Models & Routing**. Its persisted option remains
`PlaygroundSettings`; only the settings navigation location changed.

Files:

- `model/perf_metric.go`
- `model/status_check_probe_state.go`
- `pkg/perf_metrics/`
- `controller/perf_metrics.go`
- `controller/status_check_probe_task.go`
- `controller/system_task_handlers.go`
- `router/api-router.go`
- `common/constants.go`
- `model/option.go`
- `service/channel.go`
- `web/default/src/features/status-check/`
- `web/default/src/routes/_authenticated/status/index.tsx`
- `web/default/src/features/system-settings/content/status-check-section.tsx`
- `web/default/src/features/system-settings/maintenance/config.ts`
- `web/default/src/features/system-settings/maintenance/sidebar-modules-section.tsx`
- `web/default/src/features/profile/components/sidebar-modules-card.tsx`
- `web/default/src/features/system-settings/content/section-registry.tsx`
- `web/default/src/features/system-settings/models/section-registry.tsx`
- `web/default/src/hooks/use-sidebar-data.ts`

## Cap and hCaptcha integration

Cap uses a self-hosted Cap Standalone instance. Configure two Cap site keys when
login/registration and check-in use different PoW difficulties. The public
widget endpoint is `<CapServerURL>/<siteKey>/`; token verification uses
`<CapServerURL>/<siteKey>/siteverify`.

New options:

| Option | Purpose |
| --- | --- |
| `CaptchaType` | Active provider: `turnstile`, `hcaptcha`, or `cap` |
| `HCaptchaEnabled` | Enables hCaptcha |
| `HCaptchaSiteKey` / `HCaptchaSecretKey` | hCaptcha site verification key pair |
| `CapEnabled` | Enables Cap |
| `CapServerURL` | Public Cap Standalone base URL |
| `CapAdminAPIKey` | Cap API key used to update site-key difficulty |
| `CapSiteKey` / `CapSecretKey` | Login, registration, and recovery key pair |
| `CapCheckinSiteKey` / `CapCheckinSecretKey` | Daily check-in key pair |
| `LoginCaptchaDifficulty` | Login/registration difficulty, range 1-8 |
| `CheckinCaptchaDifficulty` | Check-in difficulty, range 1-8 |
| `ForceCheckinCaptcha` | Requires a fresh captcha for every check-in |
| `ForceRedemptionCaptcha` | Requires a fresh captcha for every redemption; disabled by default |
| `checkin_setting.min_user_quota` | Optional strict minimum user balance for check-in; `0` disables the gate |

Difficulty is not sent as a browser-controlled widget attribute. When a Cap
setting changes, the backend updates Cap Standalone through
`PUT /server/keys/:siteKey/config`. If the two difficulties differ, the two
flows must use different Cap site keys.

When `ForceRedemptionCaptcha` is enabled, `POST /api/user/topup` requires a
fresh token from the configured provider before a redemption code is checked.
Turnstile and hCaptcha verification bypass their login-session cache for this
route. Cap redemption uses the login/registration site key because it does not
have a separate redemption difficulty setting. The default frontend opens a
security-check dialog from the wallet redemption action and forwards the token
using the provider's existing query parameter.

`GetOptions` returns `"***"` for sensitive fields (`CapSecretKey`,
`CapAdminAPIKey`, `CapCheckinSecretKey`, `HCaptchaSecretKey`,
`TurnstileSecretKey`) that are already
set, rather than omitting them. This lets the settings form distinguish
"already configured" from "never set" so it can skip the required-field
validation and avoid blocking saves when only a boolean toggle like `CapEnabled`
changes. The frontend renders these fields empty with an explanatory placeholder
via a local `SensitiveInput` wrapper; `UpdateOption` rejects the `"***"`
sentinel if it somehow arrives, preventing accidental overwrites.

hCaptcha uses the official browser SDK at
`https://js.hcaptcha.com/1/api.js?render=explicit` and server-side verification
at `https://api.hcaptcha.com/siteverify`. The browser sends the verification
token as the `hcaptcha` query parameter; the backend also accepts hCaptcha's
standard `h-captcha-response` name for compatibility.

Email binding renders the active captcha provider before it sends a verification
email. For registration, a valid email verification code is accepted as proof
that the email-verification flow already completed the captcha, so the
registration request does not require a second human-verification challenge.
The registration form hides the completed challenge after the email is sent,
and the redemption security-check dialog includes an explicit Cancel action.

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
- `middleware/hcaptcha-check.go`
- `middleware/hcaptcha-check_test.go`
- `middleware/captcha-check.go`
- `middleware/captcha-check_test.go`
- `middleware/turnstile-check.go`
- `router/api-router.go`
- `service/cap.go`
- `service/cap_test.go`
- `web/default/src/components/cap.tsx`
- `web/default/src/components/hcaptcha.tsx`
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
- `web/default/src/features/wallet/api.ts`
- `web/default/src/features/wallet/hooks/use-redemption.ts`
- `web/default/src/features/wallet/index.tsx`
- `web/default/src/features/system-settings/auth/bot-protection-section.tsx`
- `web/default/src/features/system-settings/auth/index.tsx`
- `web/default/src/features/system-settings/auth/section-registry.tsx`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`
- `web/default/src/features/system-settings/types.ts`

Check-in balance gating is implemented in:

- `setting/operation_setting/checkin_setting.go`
- `controller/checkin.go`
- `controller/checkin_test.go`
- `controller/misc.go`
- `web/default/src/features/profile/index.tsx`
- `web/default/src/features/profile/types.ts`
- `web/default/src/features/system-settings/general/checkin-settings-section.tsx`
- `web/default/src/features/system-settings/billing/index.tsx`
- `web/default/src/features/system-settings/billing/section-registry.tsx`
- `web/default/src/features/system-settings/hooks/use-update-option.ts`

## Subscription plans and redemption codes

Redemption codes can now grant either wallet quota or a selected enabled
subscription plan. When redemption captcha is required, the frontend closes the
captcha dialog before sending the redemption request. The response remains a
number for wallet codes for backward compatibility and returns the redeemed
plan identity for subscription codes.
Subscription codes persist a wallet quota of `0` and never credit the user's
wallet balance. Startup migration normalizes any legacy subscription-code quota
to `0`, and redemption also clears it in the same transaction as a defense for
codes created outside the normal admin flow.

The redemption edit drawer exposes the same wallet/subscription type selector
as creation. Updates validate and persist `subscription_plan_id`, force quota to
`0` for subscription codes, and restore the entered quota when switching back
to a wallet code. A currently assigned disabled plan remains visible and can be
preserved, but a disabled plan cannot be newly assigned. Quantity remains a
create-only option because editing one code must not create additional codes.

Subscription plans can be deleted when they have no active user subscriptions;
expired and cancelled history does not block deletion. Each plan also has an
optional group billing policy with an enable switch, blacklist/whitelist mode,
and multi-select group list. Blacklisted groups use wallet balance instead of
that subscription. In whitelist mode, only selected groups may use the
subscription and all other groups use wallet balance. These restrictions apply
to every billing preference, and the purchase cards show **Unavailable Groups**
or **Available Groups** so users can verify eligibility before buying.

The policy is evaluated against the request's effective `UsingGroup`, not only
the user's persisted group. This keeps subscriptions correct for API keys that
select another group and for automatic routing: a request that uses a group
listed by a whitelist can consume subscription quota, while a request routed to
an ineligible group still uses the wallet. Availability checks, pre-consume,
and wallet-overflow decisions share that same effective-group rule.

Plans accept any negative quota value; the backend stores it as `-1`. This
creates a benefits-only subscription: it has no usable subscription quota and
does not participate in wallet-overflow decisions, but it can still upgrade a
user group or grant configured rate-limit entitlements. A plan can configure
one or more `{group, rpm}` entries. While that plan is active, requests in a
listed group use the highest matching plan RPM instead of the system default.
Benefits-only plans omit the **Quota per Billing Period** line from the wallet's
available plan cards and existing-subscription list. All subscription plan and
usage surfaces use this wording (Chinese: **周期内额度**) so the value is not
mistaken for an account-wide lifetime quota.

Wallet plan prices use the configured billing display currency instead of a
hard-coded dollar sign.

Files:

- `model/redemption.go`
- `model/subscription.go`
- `model/main.go`
- `controller/redemption.go`
- `controller/subscription.go`
- `controller/user.go`
- `router/api-router.go`
- `service/billing_session.go`
- `service/funding_source.go`
- `web/default/src/features/redemption-codes/`
- `web/default/src/features/subscriptions/`
- `web/default/src/features/wallet/`

## Target-user management audit attribution

Management actions that operate on a user, including quota changes, user
updates, passkey resets, and user-subscription resets, store the affected user
as the audit log owner. The logs table's user column therefore identifies the
account that changed. The administrator remains recorded under the admin-only
`other.admin_info` metadata, and `target_user_id` remains in the structured
operation parameters for traceability.

Files:

- `controller/audit.go`
- `model/log.go`
- `controller/user_manage_test.go`

## Advanced Custom Responses-to-Anthropic conversion

Advanced Custom channels now expose **OpenAI Responses to Anthropic Messages**.
The route accepts `/v1/responses`, converts the request to Anthropic Messages,
and uses `/v1/messages` with `x-api-key` authentication by default. Both
buffered and streaming Anthropic responses are converted back to the caller's
OpenAI Responses format. The public converter ID is
`openai_responses_to_anthropic_messages`; the earlier internal
`openai_responses_to_claude_messages` ID remains an alias for saved
configurations.

Files:

- `dto/channel_settings.go`
- `relay/channel/advancedcustom/adaptor.go`
- `relay/channel/claude/adaptor.go`
- `relay/channel/claude/relay-claude.go`
- `relay/channel/claude/relay_responses.go`
- `service/relayconvert/request_registry.go`
- `service/relayconvert/text_converter_registry.go`
- `web/src/features/channels/lib/advanced-custom.ts`
- `web/src/features/channels/types.ts`
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## Notice display controls

`NoticePopupMode` replaces the separate dashboard-popup switch in the default
frontend and supports `home`, `dashboard`, or `both`. The legacy
`NoticePopupOnDashboardEnabled` option and status field remain available for
older installations; a legacy enabled value maps to `both` until the new option
is explicitly saved.

`NoticeHeaderButtonMode` controls whether the top-bar announcement button opens
the existing popover below the button or a larger dialog. Both modes share the
same notice timeline, unread state, and read tracking.

Files:

- `common/constants.go`
- `model/option.go`
- `controller/misc.go`
- `web/default/src/components/notice-popup.tsx`
- `web/default/src/components/notification-popover.tsx`
- `web/default/src/hooks/use-notifications.ts`
- `web/default/src/features/system-settings/maintenance/notice-section.tsx`

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
union model list, and shows the union of groups that provide any model allowed
for the active generation tab. This keeps every usable group visible even when
the initially selected model is only available in one group. After a group is
selected, the model column is limited to that group's models that are allowed
for the active generation tab; if the current model is unavailable, the first
eligible model in that group is selected. The selection no longer relies on
effect-driven state correction when a generation tab mounts. The group column
keeps short rows at their natural height when only a few groups are eligible.
Only the combined picker is rendered, rather than separate model and group
fields.

### Model ordering and OpenAI chat relay options

Every Playground model picker sorts its options case-insensitively by model
name, with numeric segments compared numerically. This applies both to a single
group's model list and to the deduplicated list used by generation tabs.

OpenAI channel advanced settings provide two additive channel-setting flags for
Chat Completions compatibility:

| Setting | JSON field | Behavior |
| --- | --- | --- |
| **Use Responses API Only** | `use_responses_api` | Sends converted Chat Completions requests to the upstream `/v1/responses` endpoint, then converts the result back to the caller's expected format. |
| **Fake Non-Stream Support** | `fake_non_stream` | For a downstream non-streaming Chat Completions request, requests an upstream SSE response, buffers its chunks, and returns one standard JSON completion response. |

Both options require request-body conversion, so they do not override the
global or per-channel pass-through-body settings. The fake non-stream path
preserves content, reasoning content, tool calls, finish reasons, and usage
reported by the upstream stream; it estimates usage only when that stream does
not provide usage data.

Files:

- `dto/channel_settings.go`
- `relay/chat_completions_via_responses.go`
- `relay/compatible_handler.go`
- `relay/channel/openai/relay-openai-buffered.go`
- `relay/channel/openai/chat_via_responses_test.go`
- `web/default/src/features/playground/api.ts`
- `web/default/src/features/playground/hooks/use-generation-options.ts`
- `web/default/src/features/channels/`
- `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

Session-authenticated Playground relay routes are additive equivalents of the
existing token-authenticated APIs:

| Route | Purpose |
| --- | --- |
| `POST /pg/chat/completions` | Chat |
| `POST /pg/images/generations` | Image generation |
| `POST /pg/images/edits` | JSON image editing |
| `POST /pg/audio/speech` | Speech generation |
| `POST /pg/3d` | Submit a 3D task |
| `GET /pg/3d/:task_id` | Poll a user-owned 3D task |

Image controls use the OpenAI Images request shape and expose resolution and
aspect ratio as independent inputs. Resolution presets are 1K, 1.5K, 2K, 2.5K,
and 4K. Aspect-ratio presets are 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, and 2:3; users
may also enter a custom ratio as positive whole numbers in `width:height`
format. The selected resolution represents the nominal longest edge. The
frontend reduces the ratio and derives a concrete `WxH` value, using dimensions
aligned to multiples of eight when possible, so both generation and editing
continue to send the existing `size` field without introducing a new request
contract. Image editing accepts multiple reference images. Local files are
encoded as data URLs and sent as JSON in the OpenAI-compatible
`images: [{"image_url":"data:image/...;base64,..."}]` array rather than as a
multipart form. Invalid custom ratios are reported inline and block submission.

GPT Image and Seedream-compatible channels continue through the existing image
adaptors. Gemini image models, including Nano Banana aliases, are converted to
native `generateContent` image requests; uploaded edit images become Gemini
`inlineData`, and Gemini image parts are converted back to the OpenAI Images
response shape. `gpt-image-2` is included in the OpenAI model catalog.
VolcEngine image edits convert the first multipart upload to the data URI
accepted by the Seedream generations endpoint, and the channel catalog includes
Seedream 4.0 plus generic Seedream 5.0 aliases.

The image and 3D desktop layouts use the full Playground width: their fixed
control columns align with the left edge and the remaining width belongs to the
result workspace. Generated images retain their natural display size up to the
available workspace bounds, then scale down proportionally. Every generated
image also has an edit action that reuses its data URL or returned URL directly
as an `images[].image_url` reference, switches to edit mode, and returns the
user to the controls. This avoids browser-side downloads that can fail for
cross-origin result URLs.

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
not persisted separately. Deleted groups are excluded from the editor and
discarded from `GroupDefaultModel` whenever the setting is saved, with backend
filtering as a final compatibility guard.

`AutoGroupDescription` is an optional string option shown in group settings only
when `AutoGroups` is non-empty. `/api/user/self/groups` exposes the virtual
`auto` group only when the current user has at least one usable automatic group,
and uses this description when configured.

The virtual `auto` group does not need to appear in `UserUsableGroups`. When at
least one configured `AutoGroups` entry is available to the current user,
`auto` is implicitly authorized for that user. Before routing, configured auto
groups are intersected with the user's effective usable groups, preserving
configuration order while removing inaccessible and duplicate entries. The
resulting usable-group map and filtered auto-group list are cached in the Gin
request context so token authentication, Playground overrides, channel
affinity, model listing, and retry selection reuse one calculation instead of
copying and filtering settings repeatedly.

Files:

- `setting/auto_group.go`
- `constant/context_key.go`
- `service/group.go`
- `service/group_test.go`
- `service/channel_select.go`
- `middleware/auth.go`
- `middleware/distributor.go`
- `controller/model.go`
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

## Meshy2API image upload proxy

The optional Meshy2API image proxy moves base64 image payloads out of relay
requests and responses. It is disabled by default and is configured under
**System Settings -> Operations -> Worker Proxy**.

New options:

| Option | Purpose |
| --- | --- |
| `WorkerMeshyImageProxyEnabled` | Enables request and response rewriting; defaults to `false` |
| `WorkerMeshyImageProxyBaseURL` | Meshy2API service origin that provides `/upload-image` |
| `WorkerMeshyImageProxyAPIKey` | Bearer key used only by the new-api backend; masked as `***` in system settings |

When enabled, the relay recognizes image data URLs and validated bare base64
images in OpenAI chat and image requests, Responses API input, Claude image
sources, and Gemini `inlineData`. Each image is sent as the `file` part of a
multipart `POST <WorkerMeshyImageProxyBaseURL>/upload-image` request. The
original image value is then replaced with the temporary signed CDN `url`
returned by Meshy2API:

```text
https://api.meshy.ai/misc/cdn-images/uploads/{image_id}?sign={temporary_signature}
```

Meshy rejects images smaller than `32x32`. Before upload, new-api pads any
undersized edge to 32 pixels with a white background and centers the original
pixels without stretching them. Images already at least `32x32` keep their
original bytes and format.

The same conversion runs on final image output from OpenAI Images, Responses
API image-generation calls, Claude image blocks, and Gemini image parts. It is
applied to both buffered JSON and SSE data chunks. OpenAI partial-image stream
events are left untouched so previews remain incremental. A client that
explicitly requests `response_format: "b64_json"` or `"base64"` also keeps the
original base64 response contract. Non-image base64 strings are ignored.

The upload response URL is validated as HTTP(S) before it is returned to the
caller. Meshy2API does not expose a working URL-refresh endpoint, so new-api
does not register a `/v1/images/{image_id}` proxy or attempt to resolve the
returned `image_id` and `account_id` later. The CDN URL is temporary (currently
about 24 hours) and callers must consume or persist the image before it expires.
The Meshy2API key is never included in the returned URL or browser request.

The feature covers `/v1/chat/completions`, `/v1/responses`, `/v1/messages`,
OpenAI image generation/edit routes, native Gemini generation routes, and their
`/pg` chat/image equivalents. The default image Playground requests URL output
so it benefits from the proxy without retaining large base64 results in page
state. With the feature disabled or incomplete, all relay payloads retain their
previous behavior. Input upload failures stop the relay before contacting the
model provider; output upload failures log a warning and preserve the original
provider response.

The automatic rewrite explicitly excludes requests routed through a
`ChannelTypeMeshy2API` channel. Its input and output image payloads, including
base64 values, are relayed unchanged instead of being uploaded to Meshy2API and
replaced with a temporary CDN URL. This avoids sending a native Meshy2API
provider request through the Meshy2API upload proxy a second time.

Files:

- `common/gin.go`
- `controller/option.go`
- `controller/relay.go`
- `model/option.go`
- `relay/helper/common.go`
- `relay/channel/gemini/relay-gemini.go`
- `relay/channel/jimeng/image.go`
- `relay/channel/minimax/image.go`
- `relay/channel/replicate/adaptor.go`
- `router/relay-router.go`
- `service/http.go`
- `service/meshy_image_proxy.go`
- `service/meshy_image_proxy_test.go`
- `setting/system_setting/system_setting_old.go`
- `web/default/src/features/playground/components/generation/image-playground.tsx`
- `web/default/src/features/system-settings/integrations/worker-settings-section.tsx`
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

The public response keeps the upstream artifact URL internal. The content URL
uses the high-entropy public task ID as a capability: anyone holding the URL can
download the GLB without API or session credentials. The endpoint resolves the
task and streams the GLB from the original Meshy2API channel.

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

## UnrealSpeech provider

Channel type `60` adds native UnrealSpeech V8 support with default base URL
`https://api.v8.unrealspeech.com` and model name `unreal-speech-v8`. The classic
frontend remains unchanged; channel creation and Playground controls are added
to `web/default` only.

### API contract

| Route | Purpose |
| --- | --- |
| `POST /v1/audio/speech` | OpenAI-compatible binary speech response using UnrealSpeech `/speech` or `/stream` |
| `GET /v1/audio/speech/websocket?model=...` | Transparent WebSocket proxy for audio and timestamp frames |
| `POST /v1/audio/speech/tasks` | Submit a persistent long-text synthesis task |
| `GET /v1/audio/speech/tasks/:task_id` | Read the normalized public task state |
| `GET /v1/audio/speech/tasks/:task_id/content` | Authenticated audio content proxy |
| `GET /v1/audio/speech/tasks/:task_id/timestamps` | Authenticated timestamp JSON proxy |
| `POST /pg/audio/speech/tasks` | Session-authenticated Playground async submission |
| `GET /pg/audio/speech/tasks/:task_id` | Playground task polling |
| `GET /pg/audio/speech/tasks/:task_id/content` | Playground result download proxy |
| `GET /pg/audio/speech/tasks/:task_id/timestamps` | Playground timestamp JSON proxy |

For synchronous requests, `speech: true` selects upstream `/speech` and is the
default; `stream: true` selects upstream `/stream`. Both flags cannot be true.
The `/speech` adapter follows the returned temporary object URL server-side and
streams the audio bytes to the caller, so upstream S3 URLs are not exposed.
The content proxy only bypasses DNS-level SSRF rejection for HTTPS AWS S3
service hosts on port 443. This supports environments where an outbound proxy
maps public hosts into `198.18.0.0/15`; all other result URLs keep the normal
SSRF-protected fetch path.

UnrealSpeech fields accept the official capitalization (`Text`, `VoiceId`,
`Bitrate`, `Speed`, `Pitch`, `Codec`, `Temperature`, `TimestampType`) and their
lowercase equivalents. `input` and `voice` remain available for OpenAI request
compatibility. UnrealSpeech `Speed` keeps the native `-1` to `1` range.

Long text uses the existing task table and polling scheduler. Public task IDs
remain `task_*`; upstream IDs, selected multi-key credentials, and result URLs
stay in private task fields. Completed task responses expose only the gateway
`content_url`. Content fetches enforce token/session authentication, task
ownership, provider type, completion state, and SSRF policy.
When the completed upstream task includes `TimestampsUri`, the normalized
response also exposes a gateway `timestamps_url`. The route proxies the JSON
artifact with the same ownership, provider, completion, and SSRF checks, so the
temporary upstream S3 URL remains private.

### Billing

Every Unicode character is one input token (`TokenTypeTextNumber`). Synchronous
speech, asynchronous tasks, and WebSocket JSON payloads all use the existing
metered input-token billing path. UnrealSpeech handlers do not add a second
audio-output charge. The default `unreal-speech-v8` model ratio is `8.17`
(approximately `$0.01634` per 1,000 characters before group multipliers), and
operators can override it through the existing model pricing settings. Async
failures use the existing task refund lifecycle.

### Playground

`PlaygroundSettings.speech_model_types` accepts `unrealspeech`. The speech tab
then exposes the UnrealSpeech voice list, `/speech` versus `/stream`, bitrate,
native speed, pitch, and the supported output formats. Voice labels include the
language in parentheses. An `async` mode submits and polls the persistent task
API with progress feedback. At more than 1,000 Unicode characters the `stream`
option is disabled; at more than 5,000 characters the Playground selects
`async` and disables both synchronous modes. The default remains the existing
`openai` speech model type, so old settings are unchanged.

### Files

- `common/api_type.go`
- `constant/api_type.go`
- `constant/channel.go`
- `controller/audio_speech_proxy.go`
- `controller/channel-test.go`
- `controller/relay.go`
- `dto/audio.go`
- `middleware/distributor.go`
- `model/task.go`
- `relay/channel/adapter.go`
- `relay/channel/task/taskcommon/helpers.go`
- `relay/channel/task/unrealspeech/`
- `relay/channel/unrealspeech/`
- `relay/common/relay_info.go`
- `relay/constant/relay_mode.go`
- `relay/relay_adaptor.go`
- `relay/relay_task.go`
- `router/relay-router.go`
- `router/video-router.go`
- `setting/playground_setting/`
- `types/relay_format.go`
- `web/default/src/features/channels/`
- `web/default/src/features/playground/`
- `web/default/src/features/system-settings/models/playground-settings-card.tsx`
- `web/default/src/features/system-settings/models/playground-settings.ts`
- `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## Responses scalar input compatibility

Some OpenAI-compatible upstream Responses API implementations reject the
official scalar form of `input` with `Input must be a list`. The OpenAI
Responses adapter now converts a scalar string such as `"hello"` into the
equivalent user input item before sending the upstream request, while keeping
existing list input unchanged. This applies to normal converted requests;
pass-through request mode continues to forward the original body unchanged.

Files:

- `dto/openai_request.go`
- `relay/channel/openai/adaptor.go`
- `relay/channel/openai/responses_request_test.go`

## Monero wallet-RPC top-ups

Monero is an additive wallet top-up gateway. A user enters the desired system
quota amount, then the backend obtains the live Monero/USD price and creates a
unique subaddress through `monero-wallet-rpc`. The invoice stores the XMR
atomic amount, the USD/XMR price, the invoice-specific USD-to-internal-quota
conversion, and the required confirmation count. This makes the conversion
stable even if the market or administrator settings change after the user has
opened the invoice.

Selecting Monero creates the invoice immediately, and the payment dialog shows
the requested quota alongside the exact XMR principal and a QR URI. It
explicitly states that network fees are not included in the displayed
principal. Once the configured confirmation count is reached, the monitor
credits quota from the XMR amount actually received at the invoice's stored
rate. Paying the quoted principal credits the requested quota amount; a
transfer below the principal remains pending, and an overpayment credits the
additional received XMR proportionally at that same stored rate. An invoice
expires three hours after creation.

### Configuration and network boundary

Configure **System Settings → Billing → Payment Gateway → Monero** with the
wallet RPC URL, wallet-RPC credentials, network (`mainnet`, `testnet`,
or `stagenet`), one or more confirmations, and the maximum subaddress count.
The gateway is unavailable
until payment compliance is confirmed and all required Monero settings are
valid. It verifies that each newly-created subaddress belongs to the configured
network before persisting an invoice.

`MoneroUSDToCurrencyRate` is an optional Monero-only quote override. It means
**1 USD = X system-currency units** and is used when converting the chosen
wallet top-up amount to the USD value of a new XMR invoice and when freezing
its quota credit conversion. Set it to `0` to retain the normal system display
rate. For example, if a custom system currency is intentionally configured as
`1 unit = 1 CNY` for other payment gateways, set this option to the live
CNY-per-USD rate (such as `7.25`) when Monero invoices should use the real
XMR/USD conversion. The selected rate is frozen on each invoice, so later
configuration changes do not affect payment or credit settlement.

### Subaddress capacity and safe audit

`MoneroMaxSubaddresses` defaults to `10000` and counts every address in wallet
account `0`, including the primary address. Before a new invoice subaddress is
created, the backend obtains the actual count from `monero-wallet-rpc`; once
the configured limit has been reached, invoice creation is refused. The
count-and-create sequence is serialized in-process so concurrent invoice
requests cannot bypass the limit on one application node.

If wallet RPC returns a subaddress already stored by this deployment, invoice
creation rolls back both the conflicting Monero row and its generic top-up row,
then asks wallet RPC for the next subaddress. This handles restored or replayed
wallet indexes without violating the unique address constraint or leaving an
orphan top-up. Repeating the same address within one request is rejected, and
the configured subaddress-capacity check still applies to every retry.

Monero wallet RPC has no operation to delete an individual subaddress. A
completed and unlocked address is therefore **not** reused: a late payment or
overpayment cannot safely be attributed to a new invoice. While Monero payments
are enabled, the `monero_address_audit` scheduled system task runs every 24
hours. It considers only matching terminal invoice/top-up records, calls
`get_balance` with strict balances, and records counts for fully unlocked,
still locked, and wallet-unreported addresses in the system-task history. It
does not delete subaddresses, move funds, or modify invoices. When capacity is
exhausted, use a fresh receiving wallet after operationally reconciling the
old wallet; do not attempt to recycle invoice subaddresses.

Sensitive values returned by the settings API as `***` are read-only
placeholders. The default frontend renders the Waffo Pancake private-key and
Monero wallet-RPC password fields blank until an operator enters a replacement.
The Waffo Pancake save endpoint also discards that placeholder defensively, so
an older browser client cannot replace a persisted private key with `***`.

### Waffo Pancake subscription pricing

For a subscription plan with `WaffoPancakeProductId`, Waffo Pancake is the
source of truth for the checkout price. Configure and publish the product price
in Waffo Pancake, then select that product in the plan. NewAPI sends no
checkout `PriceSnapshot`, so editing the plan's `price_amount` neither changes
nor synchronizes the Pancake price; the signed completion webhook records the
actual Pancake payment amount. The optional create-product action can seed a
new Pancake product from the plan, but later checkout prices remain controlled
only in Waffo Pancake. The plan price remains available for other payment
methods.

For subscription completion webhooks, buyer identity may be either the
canonical `new-api-user-<id>` value or an email whose normalized value matches
the local order owner's email. Any other identity is rejected. An unresolved
subscription order returns a non-2xx response so Waffo Pancake can retry instead
of treating the event as successfully consumed.

The Monero completion toast uses the existing frontend i18n key
`Monero payment credited successfully`, which is present in all supported
locales.

The application does not run or require a local `monerod`. Run
`monero-wallet-rpc` against a trusted remote daemon (including a
Cake Wallet-compatible remote node) and bind wallet RPC only to a private
address. `monero-wallet-rpc` is still required because it owns the wallet,
creates subaddresses, and reports incoming transfers.

For a disposable testnet validation wallet:

```bash
monero-wallet-cli \
  --testnet \
  --generate-new-wallet /srv/monero/testnet-wallets/new-api-e2e.wallet \
  --password '' \
  --daemon-address YOUR_TRUSTED_TESTNET_NODE:28081 \
  --trusted-daemon \
  --mnemonic-language English \
  --use-english-language-names \
  --command exit

monero-wallet-rpc \
  --testnet \
  --wallet-file /srv/monero/testnet-wallets/new-api-e2e.wallet \
  --password '' \
  --daemon-address YOUR_TRUSTED_TESTNET_NODE:28081 \
  --trusted-daemon \
  --rpc-bind-ip 127.0.0.1 \
  --rpc-bind-port 18082 \
  --rpc-login new-api:USE_A_RANDOM_PASSWORD
```

Set `MoneroNetwork=testnet`, `MoneroConfirmations=1`, and the matching wallet
RPC credentials before enabling the gateway. Send testnet XMR to the generated
subaddress, wait for one confirmation, and verify that the wallet balance and
top-up record reflect the actual received atomic amount. Testnet wallet files,
RPC credentials, and daemon addresses are deployment secrets and must not be
committed.

### Compatibility and files

- Existing payment methods and the manual top-up completion path are unchanged;
  Monero invoices can only settle through wallet-RPC confirmation.
- The monitor uses the existing scheduled-task lease mechanism, so multiple
  application instances do not credit the same invoice twice.
- The daily subaddress audit uses the same scheduled-task lease mechanism and
  is read-only; its fully-unlocked result is an operational audit signal, not a
  cleanup or reuse instruction.
- `MoneroPayment` is an additive GORM model and migrates on SQLite, MySQL, and
  PostgreSQL through the normal model initialization path.
- Existing options, `TopUp`, user quota, and top-up log records remain the
  source of truth for balance accounting.

Files:

- `setting/payment_monero.go`
- `model/monero_payment.go`
- `model/system_task.go`
- `service/monero.go`
- `service/monero_test.go`
- `controller/topup_monero.go`
- `controller/topup.go`
- `router/api-router.go`
- `model/option.go`
- `controller/option.go`
- `service/waffo_pancake.go`
- `web/src/features/wallet/`
- `web/src/features/system-settings/integrations/payment-settings-section.tsx`
- `web/src/features/system-info/components/system-tasks-panel.tsx`
- `docs/monero-payment.md`

## Authentication rate-limit isolation

Login and registration no longer share the general `CT` critical-operation IP
bucket. Password login, passkey login, and 2FA verification use the dedicated
`LG` bucket; registration uses the independent `RG` bucket. This prevents
unrelated critical operations (for example, OAuth, password reset, or payment
actions) from making a normal user wait before they can sign in or create an
account.

The default deployment values are intentionally modestly more permissive while
preserving IP-based brute-force protection and the existing captcha checks:

| Variable | Default | Scope |
| --- | ---: | --- |
| `LOGIN_RATE_LIMIT` | `30` | Password, passkey, and 2FA attempts per client IP |
| `LOGIN_RATE_LIMIT_DURATION` | `600` | Login window in seconds |
| `REGISTER_RATE_LIMIT` | `20` | Registration attempts per client IP |
| `REGISTER_RATE_LIMIT_DURATION` | `600` | Registration window in seconds |

`CRITICAL_RATE_LIMIT_ENABLE=false` still disables both dedicated buckets and
the existing `CT` bucket. Keep it enabled for public deployments. Docker
Compose operators can override the four values through the service `environment`
section; the example entries are included in `docker-compose.yml`. The global
API limit and the two-per-30-second email-verification limit remain unchanged.

Files:

- `common/constants.go`
- `common/init.go`
- `middleware/rate-limit.go`
- `middleware/rate_limit_test.go`
- `router/api-router.go`
- `docker-compose.yml`

## Global upstream error rewriting

Administrators can configure global client-facing error rewrites under
**System Settings -> Operations -> Global Error Rewrite**. The feature is
disabled by default and is stored as two additive options:

| Option | Purpose |
| --- | --- |
| `error_rewrite.enabled` | Global switch for applying configured rewrites |
| `error_rewrite.rules` | JSON array of unique upstream HTTP status codes and replacement messages |

The UI exposes the rules as a table with status-code and message columns,
row-level validation, deletion controls, and an **Add Row** action. Rule
messages support these placeholders:

| Placeholder | Value |
| --- | --- |
| `{model}` | Original model requested by the client |
| `{status_code}` | HTTP status returned to the client after any channel mapping |
| `{upstream_status_code}` | Original HTTP status received from the upstream |

Rules match the original upstream status before per-channel status-code
mapping. A rewrite changes only the client-facing message; the returned HTTP
status, protocol-specific error shape, error code, retry behavior, channel
health decisions, and diagnostic logging remain unchanged. Local validation,
billing, quota, and routing failures are not rewritten. The same behavior is
applied to synchronous relay formats and asynchronous task submissions,
including video, 3D, and task-based speech endpoints.

The settings implementation uses a synchronized config codec so live option
updates cannot race with relay requests. Invalid status ranges, duplicate
codes, empty messages, non-array JSON, and invalid switch values are rejected
at the option boundary.

Files:

- `setting/operation_setting/error_rewrite.go`
- `setting/config/config.go`
- `model/option.go`
- `service/error.go`
- `relaykit/types/error.go`
- `controller/relay.go`
- `relay/relay_task.go`
- `dto/task.go`
- `web/src/features/system-settings/operations/error-rewrite-section.tsx`
- `web/src/features/system-settings/operations/error-rewrite-table.tsx`
- `web/src/features/system-settings/operations/error-rewrite-utils.ts`
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## User usage rankings

The existing `/rankings` page now keeps its Today, Week, Month, and Year
period selector while adding a Models/Users view switch on the right side of
the same control row. The selected view is persisted in the `view` URL search
parameter. User rankings render as two responsive columns with up to ten users
per column:

- **Token Usage** ranks users by total tokens and shows the group with the
  highest token use for each user.
- **Quota Usage** ranks users by consumed quota and shows the group with the
  highest quota consumption for each user.

Each row includes the current display name when configured, the username, the
selected metric total, and the metric-specific most-used group. Deleted users
and disabled accounts are excluded.

`GET /api/rankings/users?period=today|week|month|year` returns both top-ten
lists in one response. It follows the same `rankings` header-navigation access
setting as the model rankings: when anonymous rankings access is enabled, both
views are public; when login is required, both views require authentication.
No separate user-ranking visibility switch is introduced. The backend reads
the existing hourly `quota_data` rollup instead of the raw consume log, runs
the two bounded aggregate queries in parallel, and limits the group aggregate
to the union of users appearing in either top-ten list. A composite
`(user_id, created_at, use_group)` index accelerates the follow-up group lookup.
Each period snapshot is cached for five minutes, concurrent cache misses are
coalesced into one build, and an open Users view revalidates on the same
interval. This keeps the query count and returned row count bounded as the
usage history grows without leaving an open leaderboard indefinitely stale.

Files:

- `model/usedata.go`
- `model/usedata_rankings.go`
- `service/rankings.go`
- `controller/rankings.go`
- `router/api-router.go`
- `web/src/features/rankings/`
- `web/src/routes/rankings/index.tsx`
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`

## Stream first-response timeout and channel fallback

Each channel has an additive `stream_first_response_timeout` setting measured
in seconds. It applies only to streaming requests and covers the interval from
starting the upstream request through receiving the first upstream body byte.
The normal stream idle timeout still governs later gaps between chunks.

Configure it from **Channels -> Edit channel -> Advanced settings -> Stream
first response timeout**. The value is persisted inside the channel's existing
`setting` JSON, so no database migration is required:

```json
{
  "stream_first_response_timeout": 30
}
```

When the first-response deadline expires, the relay returns a retryable channel
error and the existing priority routing selects the next eligible channel. The
timeout does not automatically disable the channel. A value of `0` preserves
the previous unlimited first-response wait. Channel validation accepts values
from `0` through `86400` seconds.

Files:

- `relaykit/dto/channel_settings.go`
- `relaykit/types/error.go`
- `relay/channel/api_request.go`
- `service/channel.go`
- `controller/relay_retry_test.go`
- `web/src/features/channels/`

## Token group migration

Administrators can migrate every non-deleted token in one existing group to
another configured group from the API key page. `GET /api/token/group-names`
returns the current source groups and configured target groups;
`POST /api/token/group-migrate` performs the migration in one database
transaction and invalidates affected token cache entries after commit.

The UI entry is **API Keys -> Migrate token groups** and is visible only to
administrators. The write endpoint accepts this payload and returns the number
of migrated tokens:

```json
{
  "source_group": "ClaudeA",
  "target_group": "ClaudeB"
}
```

Both endpoints require administrator authentication. A target must be a
currently configured ratio group, or `auto` when automatic groups are enabled.
The source and target cannot be the same.

The implementation uses GORM and the shared reserved-column identifier, so the
operation remains compatible with SQLite, MySQL, and PostgreSQL. Deleted tokens
are intentionally left unchanged.

Files:

- `model/token.go`
- `controller/token_group.go`
- `router/api-router.go`
- `web/src/features/keys/`

## Managed appearance and model square defaults

The Site Settings page includes an **Appearance & Model Square** section backed
by additive `console_setting.*` options. It covers every setting exposed by the
user theme drawer: light/dark/system mode, color preset, font, radius, density,
sidebar variant, sidebar layout, content width, and text direction. It also
configures the administrator-controlled background image, the model square's
default card/table view, and the page size for each view.

The admin entry is **System Settings -> Site -> Appearance & Model Square**.
The section maps directly to these public options:

| Option | Allowed values / behavior |
| --- | --- |
| `console_setting.background_image` | Empty, absolute HTTP(S) URL, or root-relative path such as `/_custom/img/background.webp` |
| `console_setting.default_theme` | `system`, `light`, `dark` |
| `console_setting.default_theme_preset` | Any preset exposed by the theme drawer |
| `console_setting.default_theme_font` | `default`, `sans`, `serif` |
| `console_setting.default_theme_radius` | `default`, `none`, `sm`, `md`, `lg`, `xl` |
| `console_setting.default_theme_scale` | `default`, `sm`, `lg`, `xl` |
| `console_setting.default_sidebar_variant` | `inset`, `floating`, `sidebar` |
| `console_setting.default_sidebar_layout` | `expanded`, `icon`, `offcanvas` |
| `console_setting.default_content_layout` | `full`, `centered` |
| `console_setting.default_direction` | `ltr`, `rtl` |
| `console_setting.model_square_default_view` | `card`, `table` |
| `console_setting.model_square_card_page_size` | Multiple of 6 from `6` through `96` |
| `console_setting.model_square_table_page_size` | Integer from `5` through `100` |

Administrator defaults apply only when the corresponding personal cookie is
absent. A user's explicit theme choice remains pinned even when it equals the
current administrator default; resetting an axis removes that personal cookie
and returns to the administrator default. Users cannot override the background
image.

The authenticated console renders the configured image at full viewport size;
the header, sidebar, and main content use translucent blurred surfaces. The
model square and rankings use the image only in their upper hero region. Card
page sizes must be multiples of six from `6` through `96`, which keeps both the
two-column and three-column grids complete. The default is `18`; table page
sizes may range from `5` through `100` and default to `20`.

Files:

- `setting/console_setting/config.go`
- `controller/misc.go`
- `web/src/stores/system-config-store.ts`
- `web/src/context/`
- `web/src/components/layout/`
- `web/src/features/pricing/`
- `web/src/features/rankings/`
- `web/src/features/system-settings/site/`

## SPA metadata without reverse-proxy rewriting

Site Settings also exposes the document description, Open Graph type, and Open
Graph description. The Go web router parses the embedded SPA HTML and updates
those tags plus the favicon URL from the existing `Logo` setting before serving
an application route. The frontend applies the same values after `/api/status`
loads, covering separately hosted frontend deployments.

Use **System Settings -> Site -> System Information** for `Logo`, and **System
Settings -> Site -> SPA Metadata** for the metadata fields. `Logo` now accepts
an HTTP(S) URL or a root-relative path such as `/_custom/img/logo.png`. The SPA
metadata options are:

| Option | Validation |
| --- | --- |
| `console_setting.spa_meta_description` | At most 500 characters |
| `console_setting.spa_meta_og_type` | Open Graph type identifier, default `website` |
| `console_setting.spa_meta_og_description` | At most 500 characters |

The existing protected project title, `meta[name=title]`, and `og:title` remain
unchanged. This customization replaces the previous need for nginx
`sub_filter` rules for the favicon and description metadata without allowing
protected project identity to be replaced.

After deployment, request any SPA route directly (for example `/pricing`) and
inspect the returned HTML to verify the favicon, description, `og:type`, and
`og:description`. This validates server-rendered metadata independently of the
client-side runtime update.

Files:

- `router/web-router.go`
- `web/index.html`
- `web/src/lib/dom-utils.ts`
- `web/src/hooks/use-system-config.ts`
- `web/src/features/system-settings/site/spa-meta-section.tsx`

## Upstream sync checklist

1. Fetch and merge `upstream/main` on a temporary sync branch.
2. Review conflicts first in `model/option.go`, `controller/option.go`,
   `controller/misc.go`, `router/api-router.go`, the auth forms, system-settings
   registries, and `use-sidebar-data.ts`.
3. Confirm that no existing option or status field was removed or renamed.
4. Run `gofmt` on changed Go files.
5. Run `go test ./common ./model ./middleware ./service ./controller ./router`.
6. From `web`, run `bun run i18n:sync`, `bun run typecheck`, targeted
   lint for changed files, and `bun run build`.
7. Test Turnstile, hCaptcha, and Cap separately, including token reuse
   rejection and a forced check-in after an earlier login captcha.
8. Test `checkin_setting.min_user_quota` with balances equal to and greater
   than the threshold; the equal case must remain rejected.
9. Test wallet Markdown, internal/absolute custom-tab URLs with both open modes,
   vendor CRUD, and Model Square pricing with and without a selected group.
10. Test Playground history editing with an IME and verify file, image, camera,
   screen-capture, and attachment-only submissions.
11. Verify that only usage-log rows retain the fixed height and that user search
    commits after the debounce without delaying visible typing.
12. Test notice popup behavior on `/` and `/dashboard/overview`, including X
    dismissal, **Close Today** persistence, empty notices, and both option
    combinations.
13. If upstream changes stream event parsing, re-audit TTFT so lifecycle events
    are not accidentally treated as model tokens without an explicit metric
    decision.
14. On the API key page, verify the Terminal action remains visible beside copy,
    each tool config uses the selected key/group/model, and all copy actions
    include the full resolved key and normalized site URL.
15. In group pricing, verify model selectors are not clipped by the table,
    request only group-available models, and the optional auto-group description
    is returned only when automatic grouping is usable.
16. Verify a group with `GroupRetryTimes: 0` does not retry, omitted groups use
    global `RetryTimes`, and auto-group routing applies the override of the
    concrete selected group.
17. Verify passive recovery probes only auto-disabled channels and that turning
    off a channel's **Auto Ban** switch prevents both relay-triggered and
    scheduled-test automatic disabling.
18. Verify an API key assigned to `auto` is accepted whenever the user retains
    at least one configured automatic group, inaccessible candidates are never
    selected, and duplicate auto-group entries are evaluated only once.
19. With the Meshy2API image proxy enabled, verify base64 image input is
    uploaded before relay, final image output returns the temporary signed CDN
    URL from `/upload-image`, explicit `b64_json` output remains base64, and no
    Meshy2API key is exposed. For a request routed through a Meshy2API channel,
    verify both input and output payloads remain unchanged and no upload occurs.
