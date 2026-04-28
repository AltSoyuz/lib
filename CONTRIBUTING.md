# Contributing

Thanks for your interest in contributing to `github.com/AltSoyuz/lib`.

## Ground rules

1. **Keep dependencies minimal.** Any new third-party dep needs a justification
   in the PR description. Prefer the standard library.
2. **Don't break the public API silently.** Renames, signature changes, or
   removed exports require an entry under `## [Unreleased]` in `CHANGELOG.md`
   and will trigger a minor (pre-1.0) or major (post-1.0) version bump.
3. **Tests are required.** New behavior comes with `*_test.go`. Bug fixes come
   with a regression test.
4. **No god packages.** If a package starts doing two unrelated things, split
   it.

## Development

```sh
git clone git@github.com:AltSoyuz/lib.git
cd lib
go test -race ./...
go vet ./...
```

If you have [`staticcheck`](https://staticcheck.dev/):

```sh
staticcheck ./...
```

## Pull requests

- One logical change per PR.
- Commit messages: imperative mood (`add foo`, not `added foo`).
- Update `CHANGELOG.md` under `## [Unreleased]` for any user-visible change.
- CI must be green before review.

## Releases

Releases are cut by tagging `main`:

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

The `release.yml` workflow generates GitHub release notes from the tag.
Move entries from `## [Unreleased]` to a new `## [X.Y.Z]` section in
`CHANGELOG.md` as part of the release commit.
