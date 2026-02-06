# Contributing

## Standards

- Keep packages algorithm-focused and application-agnostic.
- Prefer small APIs, deterministic behavior, and explicit error handling.
- Add doc comments for exported identifiers.

## Workflow

1. Add/update tests with every behavior change.
2. Run `just ci` before pushing.
3. Keep pull requests scoped to one logical change.

## Versioning and compatibility

- Follow semantic versioning (`vMAJOR.MINOR.PATCH`).
- Before `v1.0.0`, minor releases may include breaking API changes when documented.
- At and after `v1.0.0`, avoid breaking public APIs without migration notes.
- Keep support to the latest stable Go release and the previous stable Go release.

## Release baseline

- Tag releases as `v*` (for example `v0.1.0`).
- Use prerelease semver tags for unstable snapshots (for example `v0.2.0-rc.1`).
- Tagged releases are published through GitHub Actions.
