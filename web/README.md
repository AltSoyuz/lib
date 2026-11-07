# web

The UI kit shared by the SvelteKit apps (Svelte 5, adapter-static, no CSS framework). Not a package: an app
copies this directory into `ui/src/lib/kit/` from the lib version pinned in its `go.mod`, so go.sum checks
the copy and one version drives the Go and the web side.

```make
KIT_SRC = $$(go list -m -f '{{.Dir}}' github.com/AltSoyuz/lib)/web
kit:        # after a lib bump
	rm -rf ui/src/lib/kit && cp -R "$(KIT_SRC)" ui/src/lib/kit && chmod -R u+w ui/src/lib/kit
kit-check:  # in make check: the copy is never edited by hand
	diff -r "$(KIT_SRC)" ui/src/lib/kit
```

## Contents

| File | What |
|---|---|
| `app.css` | Tokens (dark on `:root`, light on `[data-theme='light']`), base elements, native controls (a bare `input[type=search]` is the search field), buttons (`.filled`, `.tinted`, `.glass`, `.round`, `.destructive`, `.wide`), inset grouped lists (`.section > h2 + ul.list > li > .cell`), `.segmented`, `.chip-row`, `.banner`, the empty state (`.empty-text`, its `.empty-title` above the sentence). Two sizes of one look, iOS 26 and its Liquid Glass: the phone's, and from 1024px wide the same corners, capsules, grouped colors and glass with a wide screen's room (a floating glass sidebar, a 15px body text, 36px controls) and a pointer's hover, by tokens and one media query. |
| `Shell.svelte` | The page frame: `.app`, the opaque status bar strip, `main.page`, a `floating` snippet for the tab bar. |
| `TabBar.svelte` | Floating capsule on a phone, a floating glass panel from 1024px, which a button (or Cmd or Ctrl + `\`) folds to a small panel keeping the button. Needs `.tabbed` on the shell. |
| `sidebar.svelte.ts` | Whether the desktop sidebar is folded: kept by the browser, written on `<html data-sidebar>` for the CSS; `toggleSidebar()`. The button's words are `text.hideSidebar` and `text.showSidebar`. |
| `TitleBar.svelte` | Large or inline title, back button, `trailing` and `banners` snippets, then the offline and new version banners. |
| `Sheet.svelte` | Bottom sheet on a native `<dialog>`: pull down to close, Cancel/Save header with `onsave`. |
| `pwa.svelte.ts` | Online state, standalone detection, install prompt, new version check, iOS keyboard viewport fix. |
| `theme.svelte.ts` | System, light or dark per device: `initTheme(key)` once in the root layout. |
| `api.ts` | `request()` for JSON calls to the app's API, `HttpError`, `errorText()`. |
| `SignIn.svelte` | The sign-in by a 6-digit code sent by email, against lib's `backoffice` (`POST /api/v1/sign-in/code`, `POST /api/v1/sign-in/verify`): email step, code step that outlives a reload of the app, resend after 30 s; `children` goes under the email step. |
| `text.ts` | The words the kit shows outside `admin/`, in English: `useText({...})` in the root layout's script puts an app in its own language. |
| `service-worker.ts` | Offline shell, precache, push notifications: `serve({ name, icon })` from `src/service-worker.ts`. |
| `push.ts` | The phone side of the `push` package: `pushState(standalone)`, `turnOn(key)` from a tap, `turnOff()`. |
| `viewport.ts` | Pure helper of the viewport fix, tested with `node --test`. |
| `Tour.svelte`, `tour.ts` | The first-run tour as an iOS onboarding, over the whole screen: a welcome (logo, a row per thing the app does, a note), then one swiped page per step with the app's scene of its screen, a title, one sentence and Try it. The app writes `TourWelcome` and `TourStep`s, draws each scene on `.card`, `.label` and `.pill`, opens the tour and keeps its page (`onpage`, `ontry`, `onfinish`); `shownInTour(items, admin)` keeps what every account has. |
| `features.svelte.ts` | The features of lib's `backoffice`, kept on the phone: `initFeatures(key)` once in the root layout, then `featureOn(name, role)` where the app shows a feature. |
| `admin/` | The back office of lib's `backoffice` package: `admin.ts` (API, types, account labels), `Users`, `Announcement`, `AnnouncementBanner`, `Features`, `AuditLog`, `Stats`, `PendingAccess`, `ViewingAs`, and `Banners` (View as and the announcement /me asks for, for `TitleBar`'s `banners`). An app composes its `/admin` page from them and adds its own tabs. |

## Rules

- An app imports the kit as `$lib/kit/...` and overrides tokens in its own CSS, after `kit/app.css`.
- Below 1024px the web look changes nothing: a phone renders as before, which the apps check by screenshots at 390px.
- An app's own card on `--card` adds `box-shadow: var(--card-edge)` (none on a phone, a ring on the web); a field the
  app draws bare inside its own box sets `border: 0`.
- Icons are the app's: a component with `<svelte:options namespace="svg" />` rendering paths for a name, passed
  to `TabBar` as the `icon` snippet.
- A native control is drawn by the kit: an app writes it bare (`<input type="search">` is the search field: full width,
  a pill with a magnifier) and never draws its own version of one, so every app looks the same.
- Something enters the kit when a second app uses it; until then it stays in the app.
- Compatibility as for the Go packages: a change that breaks an app is a new minor version, named in CHANGELOG.md,
  and ships with the migration of the apps that copy the kit.
- What still moves (the layout under iOS) is tuned in one app and enters the kit once validated on an iPhone.
