package controller

import (
    "context"
    "testing"
    "time"

    netboxv1 "github.com/netbox-community/netbox-operator/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResearchForeignL2VPNSurvivesClaimDeletion(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    now := metav1.NewTime(time.Now())
    claim := &netboxv1.L2VPNClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-vpn", Finalizers: []string{L2VPNClaimFinalizerName}, DeletionTimestamp: &now},
        Spec: netboxv1.L2VPNClaimSpec{Type: "vxlan", IdentifierRangeStart: 1000, IdentifierRangeEnd: 2000},
    }
    foreign := &netboxv1.L2VPN{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-vpn"},
        Spec: netboxv1.L2VPNSpec{Type: "vxlan", Identifier: 1001},
    }
    client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, foreign).Build()
    r := &L2VPNClaimReconciler{Client: client, Scheme: scheme}
    key := types.NamespacedName{Namespace: "default", Name: "shared-vpn"}
    if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil { t.Fatal(err) }
    if err := client.Get(context.Background(), key, &netboxv1.L2VPN{}); err != nil {
        if apierrors.IsNotFound(err) { t.Fatal("FOREIGN_L2VPN_DELETED: deleting Claim removed an unowned same-name L2VPN") }
        t.Fatal(err)
    }
}

func TestResearchForeignIpRangeSurvivesClaimDeletion(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    now := metav1.NewTime(time.Now())
    claim := &netboxv1.IpRangeClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-range", Finalizers: []string{IpRangeClaimFinalizerName}, DeletionTimestamp: &now},
        Spec: netboxv1.IpRangeClaimSpec{ParentPrefix: "10.0.0.0/24", Size: 4},
    }
    foreign := &netboxv1.IpRange{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-range"},
        Spec: netboxv1.IpRangeSpec{StartAddress: "10.0.0.8/32", EndAddress: "10.0.0.11/32"},
    }
    client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, foreign).Build()
    r := &IpRangeClaimReconciler{Client: client, Scheme: scheme}
    key := types.NamespacedName{Namespace: "default", Name: "shared-range"}
    if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil { t.Fatal(err) }
    if err := client.Get(context.Background(), key, &netboxv1.IpRange{}); err != nil {
        if apierrors.IsNotFound(err) { t.Fatal("FOREIGN_IPRANGE_DELETED: deleting Claim removed an unowned same-name IpRange") }
        t.Fatal(err)
    }
}


func TestResearchOwnedL2VPNRemovedWithDeletingClaim(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    now := metav1.NewTime(time.Now())
    uid := types.UID("vpn-claim-uid")
    controller := true
    claim := &netboxv1.L2VPNClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "owned-vpn", UID: uid, Finalizers: []string{L2VPNClaimFinalizerName}, DeletionTimestamp: &now},
        Spec: netboxv1.L2VPNClaimSpec{Type: "vxlan", IdentifierRangeStart: 1000, IdentifierRangeEnd: 2000},
    }
    owned := &netboxv1.L2VPN{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "owned-vpn", OwnerReferences: []metav1.OwnerReference{{APIVersion: "netbox.dev/v1", Kind: "L2VPNClaim", Name: claim.Name, UID: uid, Controller: &controller}}},
        Spec: netboxv1.L2VPNSpec{Type: "vxlan", Identifier: 1010},
    }
    cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, owned).Build()
    r := &L2VPNClaimReconciler{Client: cl, Scheme: scheme}
    key := types.NamespacedName{Namespace: "default", Name: claim.Name}
    if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil { t.Fatal(err) }
    if err := cl.Get(context.Background(), key, &netboxv1.L2VPN{}); !apierrors.IsNotFound(err) { t.Fatalf("owned VPN should be deleted, got %v", err) }
}

func TestResearchOwnedIpRangeRemovedWithDeletingClaim(t *testing.T) {
    scheme := runtime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    now := metav1.NewTime(time.Now())
    uid := types.UID("range-claim-uid")
    controller := true
    claim := &netboxv1.IpRangeClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "owned-range", UID: uid, Finalizers: []string{IpRangeClaimFinalizerName}, DeletionTimestamp: &now},
        Spec: netboxv1.IpRangeClaimSpec{ParentPrefix: "10.0.0.0/24", Size: 4},
    }
    owned := &netboxv1.IpRange{
        ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "owned-range", OwnerReferences: []metav1.OwnerReference{{APIVersion: "netbox.dev/v1", Kind: "IpRangeClaim", Name: claim.Name, UID: uid, Controller: &controller}}},
        Spec: netboxv1.IpRangeSpec{StartAddress: "10.0.0.20/32", EndAddress: "10.0.0.23/32"},
    }
    cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim, owned).Build()
    r := &IpRangeClaimReconciler{Client: cl, Scheme: scheme}
    key := types.NamespacedName{Namespace: "default", Name: claim.Name}
    if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil { t.Fatal(err) }
    if err := cl.Get(context.Background(), key, &netboxv1.IpRange{}); !apierrors.IsNotFound(err) { t.Fatalf("owned IP range should be deleted, got %v", err) }
}
