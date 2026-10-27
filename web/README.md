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
| `app.css` | Tokens (dark on `:root`, light on `[data-theme='light']`), base elements, buttons (`.filled`, `.tinted`, `.destructive`, `.wide`), inset grouped lists (`.section > h2 + ul.list > li > .cell`), `.segmented`, `.chip-row`, `.banner`. |
| `Shell.svelte` | The page frame: `.app`, the opaque status bar strip, `main.page`, a `floating` snippet for the tab bar. |
| `TabBar.svelte` | Floating capsule on a phone, sidebar from 1024px. Needs `.tabbed` on the shell. |
| `TitleBar.svelte` | Large or inline title, back button, `trailing` and `banners` snippets, then the offline and new version banners. |
| `Sheet.svelte` | Bottom sheet on a native `<dialog>`: pull down to close, Cancel/Save header with `onsave`. |
| `pwa.svelte.ts` | Online state, standalone detection, install prompt, new version check, iOS keyboard viewport fix. |
| `theme.svelte.ts` | System, light or dark per device: `initTheme(key)` once in the root layout. |
| `api.ts` | `request()` for JSON calls to the app's API, `HttpError`, `errorText()`. |
| `service-worker.ts` | Offline shell, precache, push notifications: `serve({ name, icon })` from `src/service-worker.ts`. |
| `viewport.ts` | Pure helper of the viewport fix, tested with `node --test`. |
| `admin/` | The back office of lib's `backoffice` package: `admin.ts` (API, types, account labels), `Users`, `Announcement`, `AnnouncementBanner`, `AuditLog`, `Stats`, `PendingAccess`, `ViewingAs`. An app composes its `/admin` page from them and adds its own tabs. |

## Rules

- An app imports the kit as `$lib/kit/...` and overrides tokens in its own CSS, after `kit/app.css`.
- Icons are the app's: a component with `<svelte:options namespace="svg" />` rendering paths for a name, passed
  to `TabBar` as the `icon` snippet.
- Something enters the kit when a second app uses it; until then it stays in the app.
- Compatibility as for the Go packages: a change that breaks an app is a new minor version, named in CHANGELOG.md,
  and ships with the migration of the apps that copy the kit.
- What still moves (the layout under iOS) is tuned in one app and enters the kit once validated on an iPhone.
