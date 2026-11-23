# Claim teardown ownership — verified Smart PR research

Date: 2026-11-23
Upstream baseline: `aa5aae315ba910246dd0079cde28665c492d2047`
Repo: independent public research mirror (NOT a fork), GitHub-only.
No upstream issue, PR, code or comment has been created.

## Finding

In the normal reconciliation deletion branches of:
- `internal/controller/vlanclaim_controller.go`
- `internal/controller/l2vpnclaim_controller.go`
- `internal/controller/iprangeclaim_controller.go`

the code fetched a resource by namespace/name and deleted it without confirming that its controlling OwnerReference pointed to the deleting Claim. This can remove a different resource if a former child was replaced by another same-name object while the Claim's finalizer remains. The same branch is reached during an ordinary Kubernetes deletion reconcile (not only through a helper call).

Source-level context: merged upstream PR #641 separately strengthened the rule that only claim-owned resources should be deleted on a restoration hash mismatch. The teardown paths still lacked equivalent ownership checking.

## Independent Red evidence on unchanged upstream

- VLAN direct Reconcile with legal synthetic deletion state: [Run #45140344482](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45140344482): `FOREIGN_VLAN_DELETED`.
- L2VPN and IP range direct Reconcile: [Run #45141243127](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141243127): `FOREIGN_L2VPN_DELETED`, `FOREIGN_IPRANGE_DELETED`.
- Real Kubernetes API Server + etcd: [Run #45141752129](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141752129): created valid standalone VLAN and same-name Claim with finalizer, deleted Claim via Kubernetes API, ran unchanged Claim Reconcile, then observed `FOREIGN_VLAN_DELETED_REAL_API`. This confirms the test state and deletion behavior with actual API machinery, not just a fake client. It does **not** use an external NetBox backend or a production telecom deployment.

## Green evidence with minimal research fix

Branch: [research/vlan-foreign-ownership-fix](https://github.com/Johnny-Kao/netbox-operator-research/tree/research/vlan-foreign-ownership-fix)

- [Run #45141324178](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141324178): 3 foreign resource preservation checks green; existing NetBox API and IP Claim integration checks green.
- [Run #45141527501](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141527501): checks both preservation of foreign resources and deletion of properly owned VLAN/L2VPN/IP ranges; relevant controller Ginkgo integration tests green.
- [Run #45141757494](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141757494): real Kubernetes API test of foreign VLAN survives is green (5.69s), related Claim Ginkgo integration tests green.

Minimal fix in each affected controller: before `r.Delete(ctx, child)`, require `metav1.IsControlledBy(child, claim)`; when child is not owned by the deleting Claim, remove that Claim's own finalizer and preserve the unrelated child.

## Decision & remaining checks

- **Candidate:** Technically staging-eligible Smart PR: correctness, actual reconciler path, baseline Red / minimal fix Green, testing on real Kubernetes API, maintainer direction alignment.
- **Not proven:** production NetBox data deletion, exploitability, operational outage, or prevalence. Avoid overstating severity.
- **Before upstream PR:** read upstream contributing rules, check active PR collisions again, consider finalizer semantics under legitimate owner UID changes, review test placement/style, make clean staging from latest upstream and separate test/fix commits, run exact upstream CI. Request explicit permission before upstream push.
- **Parked separate research:** restoration hash ambiguity across namespace/name boundaries is reproducible but changes to hash input can break existing preserved allocations; merged PR #641 explicitly retained byte-identical hashes. Do not mix with ownership PR.

CoreFoundry contribution credit is not claimed. OSS Toolkit SOP unchanged.
