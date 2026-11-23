# NetBox Operator — independent correctness research

Date: 2026-11-23
Upstream: https://github.com/netbox-community/netbox-operator
Baseline: `aa5aae315ba910246dd0079cde28665c492d2047`
Scope: IP claim allocation and recovery after partial failure. This repository is an independent research mirror, **not** a GitHub fork. No upstream changes or disclosure.

## Source-path findings

1. `IpAddressClaimReconciler` in `internal/controller/ipaddressclaim_controller.go` calls `GetAvailableIpAddressByClaim` and then creates a Kubernetes `IpAddress` CR. This is a *read of a free IP*, not a reservation in NetBox. The initial proposed failure window (NetBox reserved IP, Kubernetes CR create failed) is thus incorrect.
2. The actual NetBox write occurs in `IpAddressReconciler` (`internal/controller/ipaddress_controller.go`) through `ReserveOrUpdateIpAddress` before a Kubernetes status patch. A failure after NetBox write but before `Status().Patch()` is the pertinent recovery window.
3. `pkg/netbox/api/ip_address.go` searches for a matching IP address before creating it and checks the restoration hash when supplied. A retry may therefore correctly converge on the existing object; a duplicate allocation is **not demonstrated**.
4. A potentially relevant edge case is an ambiguous NetBox write outcome (server committed, client timed out), followed by a Kubernetes status-patch failure or restart. Prove behavior with a fault-injection test before asserting a bug.

## Baseline validation

- `go test ./pkg/netbox/api -count=1`: PASS (2026-11-23, macOS arm64, Go 1.27.3).
- `go test ./internal/controller -run '^$'`: build/compile PASS.
- Controller Ginkgo suite: BLOCKED before tests because `bin/k8s/1.33.0-darwin-arm64/etcd` is missing. `make envtest` installed `setup-envtest`, but fetching the version index from `raw.githubusercontent.com` timed out. No tests ran. Docker daemon unavailable.

## Next gates

- Obtain the exact envtest binaries and run baseline controller suite unchanged.
- Inject NetBox create-success/Kubernetes status-patch-failure and create-committed/HTTP-timeout scenarios; assert the resource count and identity after reconcile/restart.
- Verify the restoration hash and cross-controller ownership behavior with current upstream, including deletion races.
- Exclude already-covered changes and open PRs (notably #641 and #662); no issue mining.
- Only after a reproducible Red-on-baseline / Green-on-minimal-fix result, create a separate staging branch with test and fix commits separated.

Status: RESEARCH / NO CONFIRMED BUG / NO UPSTREAM PR.
