package controller

import (
    "context"
    "testing"
    "time"

    netboxv1 "github.com/netbox-community/netbox-operator/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client/fake"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Research-only: the deleting Claim may have retained its finalizer after
// a child Create failed because a different, same-name Vlan appeared.
// The Claim must not delete a Vlan it does not own.
func TestResearchForeignVlanSurvivesClaimDeletion(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil {
        t.Fatal(err)
    }
    now := metav1.NewTime(time.Now())
    claim := &netboxv1.VlanClaim{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: "default",
            Name: "shared-vlan",
            Finalizers: []string{VlanClaimFinalizerName},
            DeletionTimestamp: &now,
        },
    }
    unrelated := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-vlan"},
        Spec: netboxv1.VlanSpec{Vid: 101},
    }
    client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, unrelated).Build()
    reconciler := &VlanClaimReconciler{Client: client, Scheme: scheme}
    key := types.NamespacedName{Namespace: "default", Name: "shared-vlan"}
    if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
        t.Fatalf("reconcile: %v", err)
    }
    got := &netboxv1.Vlan{}
    if err := client.Get(context.Background(), key, got); err != nil {
        if apierrors.IsNotFound(err) {
            t.Fatal("FOREIGN_VLAN_DELETED: deleting Claim removed an unowned same-name VLAN")
        }
        t.Fatalf("lookup: %v", err)
    }
}
