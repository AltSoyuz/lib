# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `web/Tour.svelte` and `web/tour.ts`: the first-run tour of Compass, shared. A welcome (logo, rows of what the app does, a
  note), then one swiped page per step with the app's scene, a title, one sentence and Try it; dots, Skip, Continue and
  Get started. The app gives the content (`TourWelcome`, `TourStep`), a `scene` and an `icon` snippet, the page to
  start at, and keeps the page and where the tour leads (`onpage`, `ontry`, `onfinish`). Scenes draw on `.card`,
  `.label` and `.pill`, styled by the stage. `shownInTour(items, admin)` keeps what every account has: a step or a row
  of a feature kept to the admins stays out, for the admins too, and an `admin` step shows to the admins only.

## [0.5.0]

### Added

- `backoffice`: features an admin keeps to the admins, such as a beta. An app lists them in `Config.Features` (name
  and label); each is for everyone until an admin picks Admins in the back office. `Mount` serves
  `GET /api/v1/features` without a session, so an app used signed out hides them too, and
  `PUT /api/v1/admin/features/{name}`, audited as `audit.FeatureSet`. The audience is stored in a new `features`
  table (`backoffice/schema.sql`); an app that declares no feature never reads it and needs no migration.
- `web/features.svelte.ts`: `initFeatures(key)` in the root layout shows the features kept on the phone, offline
  included, then the server's; `featureOn(name, role)` says whether to show one, an admins feature only once /me gives
  the admin role. `web/admin/Features.svelte` is the back office's Features section, one Everyone or Admins picker
  per feature.

## [0.4.2]

### Changed

- `web/app.css`: chips are the filter chips of Apple's iOS 26 apps, measured on Shazam's library: 44px capsules 10px
  apart, body text, on `--chip` (a new token, lighter than the page in dark mode, white in light mode) instead of a
  34px fill. A chip of `.chip-row` is a button or a label, the latter over an invisible `<select>` for a menu.

## [0.4.1]

### Fixed

- `web/TitleBar.svelte`: the controls under a title (a search field, a segmented control) start 8px below the bar,
  19 points under the title's baseline as in Shazam. A 44px glass button of the bar touched the search field below
  it on an iPhone. A title bar without controls keeps its height.

## [0.4.0]

### Added

- `web/app.css`: `.glass`, a button on Liquid Glass for the controls of a bar, a sheet's header or a screen's
  overlay, and `.round`, an icon alone in a 44px circle. A `.filled` among the trailing controls of a title bar is
  as tall as they are, 44px.

### Changed

- `web/`: the look of Apple's iOS 26 apps, measured on Shazam. Large titles are heavy (800) and sit 4px higher.
  Section headers of grouped lists (`.section > h2`) are bold, 22px, in the text color and aligned with the list's
  edge, no longer gray uppercase footnotes inset like the cells: an app's headers show as written, so they are written
  in sentence case. Cards and lists round at 26px, the tab bar capsule is 62px tall. Content scrolls under a soft edge
  below the status bar and an inline title bar instead of a hairline. The back button of an inline title bar is
  `.glass.round`, its sides are as wide as their buttons (a wide Done no longer overflows onto the title, which stays
  centered while there is room), and the trailing controls of a title bar are 8px apart. A sheet's header buttons are
  44px capsules 12px from its corners: Cancel or Close `.glass`, Save `.filled`, the close button of a read only sheet
  `.glass.round`. A large title still starts at `--page-top`, below the blur iOS draws under the status bar of a home
  screen app, where a native app draws it about 24 points higher.

## [0.3.7]

### Fixed

- `web/Shell.svelte`: in an iPhone home screen app, a page left while its field still had the keyboard (a sheet's
  Save that navigates, a sign-in code) kept the viewport the keyboard had shortened, so the tab bar and the other
  fixed controls sat too high over an empty band. WebKit sends no focusout for a field that leaves with its page, the
  cue `pwa.svelte.ts` remeasures on; the shell now blurs the focused field in `beforeNavigate`, as a native app
  closes its keyboard. Tuned in compass (v0.7.35) and checked on an iPhone. The shell imports `$app/navigation`.

## [0.3.6]

### Fixed

- `web/TabBar.svelte`: on a phone, a long label (compass's Recordings among five tabs) crossed the round corners of
  the selected tab's pill. Labels are 10px, the size of an iOS tab bar's, and a tab is never narrower than its label
  plus the 8px the pill's corners take on each side at the label's baseline; the other tabs share the rest.

## [0.3.5]

### Security

- `auth/sessions.ReadCookie`: on HTTPS outside localhost, where `SetCookie` writes `__Host-session`, it reads only
  that cookie and no longer falls back to `sid`. A page on any sibling subdomain can set a `sid` cookie for the parent
  domain, and the fallback let it sign a signed-out visitor into the page owner's account. A `__Host-` cookie cannot
  be set from another host. Over plain HTTP and on localhost nothing changes.

## [0.3.4]

### Fixed

- `web/Sheet.svelte`, `web/app.css`: in light mode, an open sheet left a light band fading into its backdrop at the top
  of an iPhone home screen app. iOS tints the blur it draws under the status bar from the opaque strip at the top,
  never from the backdrop, so it kept the undimmed page's color. While a sheet is open, the status bar strip (and the
  top of an inline title bar) takes `--dimmed`, the page's color under the backdrop, and the backdrop starts below
  the strip. The sheet's `<dialog>` has the class `sheet`.

## [0.3.3]

### Fixed

- `httpserver.ServeSveltekitStaticFiles`: a missing file under `/_app/immutable/` answers 404 with
  `Cache-Control: no-store` instead of inheriting the year-long immutable cache of the hashed assets. A client that
  asked for a file of the previous build during a deploy kept that 404 for a year, a white screen on every launch.

## [0.3.2]

### Fixed

- `web/Sheet.svelte`: a sheet's top stays below the large title line (`--page-top` plus 16px) instead of 10px under
  the status bar, out of the blur iOS draws there in a home screen app.

## [0.3.1]

### Fixed

- `web/TitleBar.svelte`: an inline title bar starts at the height of a large title (`--page-top`), below the blur
  iOS draws under the status bar of a home screen app; it sat 16 points higher, in that blur.

## [0.3.0]

### Added

- `audit`: who changed what, in an app's `audit_events` table (`audit/schema.sql`): `Write` in the transaction of
  the change, `After` once it committed, `List` and `Since` for a back office, the account action codes.
- `backoffice`: the back office the apps share, on lib's auth tables plus `backoffice/schema.sql` (role, access
  state, pending_since, View as sessions, the announcement): `Check`/`CheckAllowPending`/`RequireAdmin` on every
  request (pending accounts get 403 `access_pending`, a View as is read only), `Admit` and `AuditSignIn` for an
  app's sign-in flows, `EndSession` for sign out, `MeOf` for `/me`, and `Mount` for `/api/v1/admin/*` (accounts,
  roles, restrict, approve, View as, announcement, audit log, account stats).
- `web/admin/`: the UI of the back office (accounts, announcement, audit log, stats, pending page, View as banner).

### Changed

- `web/TitleBar.svelte`: a gap between the last banner and the controls under the title.

## [0.2.0]

### Added

- `web/`: the UI kit shared by the SvelteKit apps (tokens and base CSS, `Shell`, `TabBar`, `TitleBar`,
  `Sheet`, PWA state, theme, API calls, service worker). Apps copy it into their `src/lib/kit/` from the lib
  version in their `go.mod`; see `web/README.md`.

### Changed

- `httpserver`: `ServeSveltekitStaticFiles` answers `/service-worker.js`, `/_app/version.json` and
  `*.webmanifest` with `Cache-Control: no-cache` (and the manifest content type), so an app always sees
  its new version. Apps that set these headers themselves can drop that code.

## [0.1.4]

### Changed

- `httpserver`: `WriteError` no longer logs a request its client canceled (a reload, a page left
  mid-load) as an error. It writes nothing, since nobody reads the answer, and logs an info line,
  `http request canceled by the client`, with the status, method, path and error. A request whose
  context ran out of time is still logged as an error.

## [0.1.3]

### Added

- `oauthgoogle`: working Google Sign-In implementation (authorization-code
  flow with PKCE, stdlib only): `Config`, `New`, `Configure`, `NewState`,
  `NewVerifier`, `ProviderName`; `Start` and `Callback` now perform the real
  redirect and token exchange with ID token claim validation.

### Changed

- `oauthgoogle`: `Start` now returns an `error` (previously no return value).

## [0.1.2]

### Added

- `auth/sessions`: `SessionPolicy`, `NewSessionPolicy`, `NextSessionExpiry` for session TTL and refresh logic.
- `auth/sessions`: `ParseAllowlistCSV` for parsing comma-separated email allowlists.
- `httpserver`: `WriteData` and `Heartbeat` helpers for Server-Sent Events.
- `provision` package added to module (VM provisioning model, templates, renderers, validation).

### Changed

- Replaced package README documentation with godoc-first package comments.
- Corrected package descriptions in repository and AI assistant documentation.

### Fixed

- `httpserver`: fixed data race in `TestHeartbeat` — body read now happens after goroutine stops.

## [0.1.0]

### Added

- Initial release.
- Packages: `auth` (passwords, sessions, magiclinks, schema), `buildinfo`,
  `claude`, `db`, `envflag`, `httpserver`, `logger`, `mailer`, `oauthgoogle`,
  `telegram`.
