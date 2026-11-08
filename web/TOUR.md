
# App tour on lib's kit

The kit draws the tour; the app writes what it says, draws its scenes, decides when it opens and what it remembers.

Two references, one per way of remembering. Read the one closest to your app before writing anything:

- **Compass**, used without an account, remembers on the phone: `~/Workspace/dev/compass/ui/web/src/lib/tour.ts`,
  `routes/(app)/tour/+page.svelte`, Home's `Continue the tour`.
- **Explore**, everyone signed in, remembers "seen" on the server (`users.news_seen`, shared with What's new) and only
  the page to resume on the phone: `~/Workspace/dev/exploreassistant/ui/src/lib/tour.ts`, `tour.test.ts`,
  `routes/tour/+page.svelte`, the opening effect in `routes/+layout.svelte`, `e2e/onboarding.test.ts`.

## 1. Read the kit, not your memory

- `src/lib/kit/README.md` (row `Tour.svelte, tour.ts`), then `src/lib/kit/Tour.svelte` (props) and
  `src/lib/kit/tour.ts` (`TourWelcome`, `TourStep`, `TourRow`, `shownInTour`). The kit is copied by `make kit` from
  the lib version in go.mod: if `Tour.svelte` is missing, the app needs a lib bump first (a dedicated task).
- Never edit `src/lib/kit/`: a change to the tour's frame goes to lib (`web/`), with a release.
- If the app already has an onboarding (a sheet, a modal), the kit's tour replaces it. Keep whatever else shared its
  mechanism (a What's new sheet, a seen flag on the server), and spare its users: someone who saw the old tour must
  not get the new one on arrival. Update the docs that name the old mechanism (glossary, AGENTS.md).

## 2. Choose what the tour shows

List the app's main features from its tab bar, Home and docs. Then:

- **Welcome rows**: 3 to 5, one per thing the app does, `title` in 2 to 4 words, `text` in a few words. Icons are
  names the app's `IconPaths` already draws.
- **Steps**: one per main feature, 3 to 6, in the order a new user meets them. A step or a row whose feature hides
  behind a back office feature carries `feature: '<name>'` (a name of `backoffice.Config.Features`); a feature for a
  role (`admin: true`) goes last.
- Never a step for a feature the app does not ship, a setting, a page of help, or the sign-in.

## 3. Write it short

- **Title**: the feature as a verb or a noun, 2 to 4 words ("Score a turn", "Shared accounts").
- **Lead**: what the person gets, in one or two short sentences of about 90 characters at most: three lines at 32
  characters, or Try it falls under the dots on a small phone. Never how to use the screen: no "Tap X", no list, no
  bold, no step numbers.
- **Try it**: a label in 1 to 3 words and the `href` of the real screen where the feature starts.
- **Note** (welcome, optional): one line a newcomer must know, such as what works without an account or who sees
  what they share; none rather than filler.
- **Logo**: the full app icon (drawn at 72px, rounded 22%, with a shadow), not a glyph.
- Copy in the app's language (English for the AltSoyuz apps), discipline-neutral where the app's rules say so.

## 4. Draw the scenes

One `{#snippet scene(step: TourStep)}` in the tour page, a branch per `step.id`:

- The feature's screen, drawn: a `.card` (a `.label` on top) holding the one thing the feature produces (a score and
  its curve, a ranking, who pays whom), and at most one `.pill`, bubble or overlay for its second idea. The kit's
  stage styles `.card`, `.label` and `.pill`; the rest is scoped CSS in the page, named after the step
  (`.score .value`, `.ranking li`). Never reset `.card p` margins: it overrides `.label`.
- A pill hangs off its card's corner (the card `position: relative`, the pill at negative offsets), never at a
  percentage of the stage: the stage shrinks to 220px on a small phone and a placed pill then covers the card.
- Made-up people and data with consistent names (Ava, Mia, Leo...), never the phone's or the account's own data, and
  coherent between scenes (one currency, one city).
- Built from the app's tokens and components, never an image or a screenshot: those go stale, weigh, and ignore dark
  mode. Without an `Icon` component, wrap `IconPaths` in `<svg viewBox="0 0 24 24">` with stroke styles. Resizing a
  component inside a scene needs a selector stronger than its own (`.pill .faces :global(.avatar)`).
- Inert: no control in a scene. An animation stops under `prefers-reduced-motion`.

## 5. Wire it

- `src/lib/tour.ts`: `WELCOME: TourWelcome`, `STEPS: TourStep[]`, what the app remembers, and the title of a page for
  Home (page 0 is the welcome, page i + 1 the step i).
- `routes/tour/+page.svelte` (or in the app's group): `<Tour {welcome} {steps} {start} {scene} {icon} onpage ontry
  onfinish />` with `steps = shownInTour(STEPS, role === 'admin')` and
  `welcome = { ...WELCOME, rows: shownInTour(WELCOME.rows, false) }`; `icon` renders the app's `IconPaths` for a
  name, as for `TabBar`. `shownInTour` reads the features the root layout loads with `initFeatures`: an app that
  declares no feature needs neither.
- The layout treats `/tour` as full screen: no tab bar, no floating button under it.
- **What it remembers**: `onpage` keeps the page, `ontry` keeps the next page (the tour is over after the last) then
  goes to `step.try.href`, `onfinish` ends it and goes Home.
  - Without accounts: localStorage `<app>.tour`, absent (never opened), a page number, or `done`.
  - With accounts: "seen" on the server, recorded when the tour opens by itself, so it opens once and follows the
    person across devices; an existing seen or news field that is empty only for newcomers spares the others with no
    migration. The page to resume stays on the phone.
- **When it opens**: once, when someone who never saw it lands on Home, replacing the history entry. From Home's
  `onMount` when the app needs no account; from the layout's effect on `/me` when it does. Never during View as, for
  an account waiting for approval, or on a sign-in page.
- A row to replay it, with the wording the app already uses if it has one ("Tour of the app"), else "Tour", in Profile
  or the account sheet; and on Home, while a page is kept, "Continue the tour" with "n of N · title".

## 6. Test it

- A unit test next to `tour.ts` keeps it true: every step has a scene, every Try it points to an existing route, every
  icon is one `IconPaths` draws.
- The e2e harness marks the tour done (or seen) for every new phone, or each scenario lands on it. When opening
  depends on the server, one e2e file covers: opens once from Home, replays, Try it then Continue the tour, nothing
  under View as. The tour renders every page at once: assert on the dots' `aria-current` and on URLs, never on a
  text the next page also shows.
- Screenshots of every page at 390x844, light and dark, and once at 375x600 (the logo drops, the stage shrinks): a
  throwaway script in /tmp with playwright-core from `ui/node_modules`, `chromium.launch({ channel: 'chrome' })`,
  `reducedMotion: 'reduce'` so Continue moves at once, the theme set through the app's localStorage key
  (`<app>.theme`), against a server built from the branch on a temporary database. Look at each one: nothing cut,
  nothing under the dots, the scene reads at a glance. Chrome here falls back to a monospace font: text wraps worse
  than on an iPhone, so a page that fits here fits there.
- With a feature set to Admins in the back office, its step and its row vanish, for an admin too.
- The app's own checks (`npm run check`, its unit tests, lint where it exists) and a changelog entry under
  Unreleased: what the tour shows, how existing users are spared it, rollback.

## Keep it true

A feature shipped, renamed or removed changes the tour in the same change: its step, its row, its scene. A tour
that shows what the app no longer does is worse than no tour.
