# NB-05 — claim teardown ownership: production relevance gate

Date: 2026-11-23. GitHub-only independent research, NOT an upstream PR.

## Finding
The deletion branches of `VlanClaimReconciler`, `L2VPNClaimReconciler`, and `IpRangeClaimReconciler` look up child resources solely by namespace/name and call `Delete` without verifying the child's controlling owner reference. A similarly named standalone resource may therefore be deleted by a Claim that never owned it.

## Is this reachable via real Kubernetes operations?
YES as a controlled workflow, without manually fabricating a DeletionTimestamp:
1. Create a Claim with its standard finalizer.
2. Create a child VLAN with a real controlling OwnerReference.
3. Delete the child through the Kubernetes API and replace it with a standalone VLAN using the same namespace/name.
4. Delete the Claim through the Kubernetes API.
5. Resume/invoke the Claim Reconciler. The now-deleting Claim's finalizer persists and its deletion branch deletes the replacement VLAN.

The test runs against actual Kubernetes API Server + etcd. Reconciler invocation is explicit and background manager reconciliation is not running during the replacement sequence. It models an interrupted/delayed operator, NOT proof that this sequence is common during uninterrupted operation.

Baseline [GC #45163115717](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45163115717):
`REAL_LIFECYCLE_FOREIGN_VLAN_DELETED: replacement standalone VLAN removed during Claim teardown` — expected RED.

Research fix [GC #45163120957](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45163120957):
`TestResearchRealLifecycleReplacementBeforeClaimDeletion` passes (6.25s). Both workflows pass their expected assertions and other existing relevant tests.

Earlier:
- Real API direct foreign VLAN deletion baseline RED [#45141752129](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141752129) / fixed GREEN [#45141757494](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141757494).
- L2VPN + IpRange direct Reconcile RED [#45141243127](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141243127), Green [#45141324178](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45141324178).
- Positive controls show genuinely owned VLAN/L2VPN/IpRange still deleted after the guard.

## Why could maintainers miss this?
- Happy-path tests do not replace owned children with unrelated same-name objects in a deletion window.
- VLAN Claim was added in PR #618 merged 2026-11-19, L2VPN in #607 merged 2026-09-21. IpRange is older (PR #130). These controllers share the same pattern.
- Merged upstream PR #641 already implemented ownership checking for a different hash-mismatch deletion case, a relevant maintainer precedent.

## Severity and limitations
- Reachable with normal Kubernetes API operations, but requires a specific overlapping lifecycle (e.g., Operator stopped or delayed while the child is deleted/recreated). Common production frequency not measured.
- Kubernetes GC will not undo an explicit operator Delete of a foreign standalone object; actual NetBox impact depends on the object's status, finalizer, `preserveInNetbox`, and reconciliation timing. No external NetBox record deletion demonstrated.
- Small three-controller fix: before deleting child, verify `metav1.IsControlledBy(child, claim)`; on mismatch, remove only the Claim's finalizer.
- One further question: whether a live running manager can naturally execute this sequence without deliberate pause and whether a populated NetBox resource would be deleted too.
- Good potential Smart PR, not an assured upstream acceptance; no security severity label, no production outage claim.

## Research decisions
A (teardown ownership): ACTIVE, further impact/reachability tests before upstream PR.
B (ambiguous restoration hash): PARKED because migration/backward compatibility is material (merged PR #641 explicitly retained old hash bytes); no code change authorized.

No local files, no upstream action, no OSS Toolkit SOP update.
