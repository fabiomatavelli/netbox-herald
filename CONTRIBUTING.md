# Contributing to netbox-herald

Thanks for your interest in contributing! Changes are merged via pull
request — nobody, including the maintainer, pushes directly to `main`.

## Workflow

- **Maintainer**: create a topic branch directly in this repo, commit using
  [Conventional Commits](https://www.conventionalcommits.org/)
  (`<type>[optional scope]: <description>`), e.g.
  `feat(nodes): add VirtualMachine mapping support`, and open a pull
  request against `main`.
- **External contributors**: fork this repo to your own account, create a
  topic branch there, commit using Conventional Commits as above, and open
  a pull request from your fork's branch against
  `fabiomatavelli/netbox-herald:main`.

## Local development

Requirements: Go (see `go.mod` for the minimum version), Docker, `kubectl`,
and a Kubernetes cluster to test against (e.g. [kind](https://kind.sigs.k8s.io/)).

```sh
# Regenerate CRD manifests and deepcopy code after editing api/v1alpha1/*_types.go
make manifests generate

# Run unit + envtest/Ginkgo integration tests
make test

# Lint
make lint

# Build the manager binary
make build

# Regenerate the Helm chart in dist/chart after any config/ or Makefile change
kubebuilder edit --plugins=helm/v2-alpha
```

### Testing against a real NetBox

`docker/docker-compose.yml` stands up a local NetBox instance for
integration and e2e testing:

```sh
docker compose -f docker/docker-compose.yml up -d
```

## Pull requests

- Keep PRs scoped to a single logical change — see the phased build-out
  order in the [README's roadmap](README.md#roadmap) for how this project is
  being landed incrementally.
- Run `make manifests generate lint test` before opening a PR; CI re-runs
  these plus integration/e2e tests and a docs-freshness check.
- If you change `api/v1alpha1/heraldconfig_types.go`, run `make docs` and
  commit the regenerated `docs/api-reference.md` — CI fails PRs with stale
  generated docs.
