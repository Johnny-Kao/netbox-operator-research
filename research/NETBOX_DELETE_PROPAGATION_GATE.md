# Verified NetBox DELETE propagation from foreign VLAN claim teardown

Date: 2026-11-23
Status: A ACTIVE, technical Smart PR candidate; B restoration hash PARKED.
Execution: GitHub only, public independent Research repository. No upstream changes.

## Question
When a VLAN claim deletes a same-name but **unowned** Vlan CR that already represents a NetBox VLAN, does the downstream `VlanReconciler` issue the external NetBox DELETE request?

## Test
`internal/controller/research_netbox_delete_propagation_test.go`: starts real Kubernetes API/etcd (envtest), creates standalone Vlan with Vlan finalizer `vlan.netbox.dev/finalizer`, `preserveInNetbox=false`, and status `VlanId=4242` (representing a committed NetBox allocation). Creates an unrelated same-name VlanClaim with its finalizer, deletes the claim, runs its actual `VlanClaimReconciler`, then runs `VlanReconciler` when the foreign VLAN acquires a DeletionTimestamp. The NetBox `IpamVlansDestroy` client endpoint is mocked and instrumented; it returns HTTP 204 so the production deletion path completes.

### Original upstream baseline
- [GitHub Actions #45172559294](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45172559294) PASS for *expected RED assertion*:
  `NETBOX_FOREIGN_VLAN_DELETE_CALLED: VLAN Controller issued 1 DELETE request(s) for unrelated NetBox VLAN ID 4242`.
- Means actual production controller code called the NetBox DELETE client method for the foreign resource; not an external server integration.

### Minimal research fix
- [GitHub Actions #45172565086](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45172565086) GREEN:
  `TestResearchForeignVlanNetboxDeletePropagation` PASS (6.33s); foreign Vlan not in deletion state, no NetBox DELETE call, and existing related Ginkgo suites passed.
- Guard: `metav1.IsControlledBy(child, deletingClaim)` before deletion, otherwise remove only claim finalizer.

## Interpretation
This is a credible potential external data-loss path when a claim teardown finds a same-name foreign Vlan that has an existing NetBox record ID, its own finalizer, and `preserveInNetbox=false`. Kubernetes garbage collection will not undo an explicit DELETE issued by a controller. The test proves an outbound DELETE API invocation (mocked) under an actual Kubernetes API lifecycle; **it does not establish** real NetBox server deletion, production incidence, or security exploitability.

This is stronger than the earlier foreign-CR-only RED/GREEN, and aligned with ownership protection in merged upstream PR #641.

## Remaining gate
Before any upstream PR: confirm the external NetBox HTTP DELETE integration with a disposable test instance if feasible on GitHub Actions; assess whether a populated resource would be marked for teardown during ordinary controller scheduling, maintainers' contribution conventions and active PR collision. Use a clean staging branch and separate test/fix commits only with explicit upstream authorization. Never claim a production incident.

Research only, do not modify CoreFoundry contribution count or OSS Toolkit SOP.
