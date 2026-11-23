# Manager restart auto-reconciliation — production relevance gate

Date: 2026-11-23. GitHub-only, public independent research repository. Upstream remains untouched.

## Question
Does the foreign VLAN deletion require an investigator to directly call Reconcile, or will an actual controller-runtime Manager automatically exercise the vulnerable path after an operator restart?

## Test
`internal/controller/research_live_manager_restart_test.go` runs Kubernetes API Server + etcd (envtest) and:
1. Uses real Kubernetes API to create a valid VLAN Claim with finalizer and its genuinely owned VLAN child.
2. While no controller manager is running (operator downtime), deletes the child, creates an independent same-name VLAN carrying the VLAN finalizer, and deletes the old Claim.
3. Starts a real `ctrl.NewManager` with `VlanClaimReconciler.SetupWithManager`. There is **no manual call** to `Reconcile`.
4. Uses direct Kubernetes API reads to detect whether the manager marks the foreign VLAN for deletion, or safely removes the stale Claim finalizer.

## Evidence
- Unmodified upstream controller: [GitHub Actions #45186655320](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45186655320), expected baseline RED: `MANAGER_RESTART_FOREIGN_VLAN_DELETED: live controller-runtime manager marked standalone VLAN for deletion after restart`.
- With the ownership guard: [GitHub Actions #45186662475](https://github.com/Johnny-Kao/netbox-operator-research/actions/runs/45186662475), test PASS (6.83 s). Standalone VLAN remains and old Claim finishes deletion.
- Both workflows completed successfully according to their expected RED / GREEN outcomes and ran their other baseline checks.

## Meaning
**Confirmed**: if a child is deleted/replaced with an unrelated same-name resource during an operator outage/delayed reconciliation and its old Claim has not finished deletion, restarting the actual controller will exercise the bug automatically. This is not a manually invoked reconciliation-only artifact.

**Not confirmed**: (a) reboot/rolling upgrade alone, with no intervening resource replacement, triggers it; (b) a totally uninterrupted controller has a realistic timing window; (c) how often administrators or other automation recreate the same name while controller is stopped; (d) production incidents; (e) live external NetBox server deletion (the separate API propagation test proves outbound DELETE call with mock).

Assessment: low-frequency but credible lifecycle race / resource ownership correctness bug, with demonstrated NetBox Delete API propagation under the required populated-status/finalizer prerequisites. Technically worthwhile Smart PR for a small defensive ownership guard. Do not label as a critical security issue.

B restoration-hash collision remains PARKED. No upstream issue or PR, no local files.
