# IP Address reconciliation recovery — source-level audit

Date: 2026-11-23
Upstream baseline: `aa5aae315ba910246dd0079cde28665c492d2047`
Environment: GitHub only; no local execution.

## Exact flow

1. `IpAddressClaimReconciler` calls `GetAvailableIpAddressByClaim`: retrieves an available address but does **not** reserve it in NetBox.
2. It creates a child Kubernetes `IpAddress` CR with owner reference.
3. `IpAddressReconciler` invokes `ReserveOrUpdateIpAddress`.
4. `ReserveOrUpdateIpAddress` first queries NetBox by address. When no address exists, it creates the NetBox object; when one exists, it checks restoration hash (if supplied) and updates the same object.
5. Then `IpAddressReconciler` patches `Status.IpAddressId`, `Status.IpAddressUrl` and `Status.LastUpdated`.

Source:
- `internal/controller/ipaddressclaim_controller.go` (approx. 92-170)
- `internal/controller/ipaddress_controller.go` (approx. 166-220)
- `pkg/netbox/api/ip_address.go` (approx. 34-104)

## Hypothesis checks

| Failure injection | Source prediction | Status |
| --- | --- | --- |
| K8s child CR creation fails, prior to NetBox write | No NetBox IP created in this code path | Source supported, not run |
| NetBox create succeeds; K8s Status patch fails | Retry finds same IP by address; may update it instead of creating another | Plausibly safe; must run |
| NetBox create committed but HTTP response times out | Retry performs address lookup before another POST | Plausibly safe; must run |
| NetBox has more than one matching address across VRFs | `getIpAddress` takes `Results[0]` with no VRF selector | Requires semantic verification; possible collision with VRF PR #637 |

No confirmed bug. Do not submit upstream changes based on these hypotheses.

## Cloud CI execution and blocker

Branch: `research/ip-claim-recovery`
Workflow: `.github/workflows/ip-recovery-research.yml`
Commit: `c110ccf4271b88e025826fe5af3d43e91f9e0260`
Run: https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45136028768

Conclusion: **failure before any steps**. The job completed in about three seconds, `steps=[]`, `runner_id=0`, `runner_name=""`; there are no Go/test logs. This is a runner-provisioning/account/policy blocker, not evidence of a code failure. Exact GitHub cause is unconfirmed.

## Next requirements

1. Establish why the private repository cannot provision a GitHub-hosted runner. Do not change repository visibility without authorization.
2. When cloud runner executes, run baseline and add fault-injection tests preserving NetBox object identity and one-record count.
3. Only then promote a reproducible correctness problem to a prospective Smart PR.

Do not edit OSS Toolkit SOP; do not create fork or upstream PR.