package controller

import (
    "context"
    "fmt"
    "path/filepath"
    "runtime"
    "testing"

    netboxv1 "github.com/netbox-community/netbox-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    k8sruntime "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

// This test uses only real Kubernetes API operations. The initial state is a
// Claim with a controller-owned child and the standard Claim finalizer.
// The child is deleted and a standalone object takes the same namespaced
// name before the Claim is deleted.
func TestResearchRealLifecycleReplacementBeforeClaimDeletion(t *testing.T) {
    ctx := context.Background()
    env := &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
        BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
            fmt.Sprintf("1.33.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
    }
    cfg, err := env.Start()
    if err != nil { t.Fatal(err) }
    defer func() { if e := env.Stop(); e != nil { t.Error(e) } }()

    scheme := k8sruntime.NewScheme()
    if e := netboxv1.AddToScheme(scheme); e != nil { t.Fatal(e) }
    if e := corev1.AddToScheme(scheme); e != nil { t.Fatal(e) }
    cl, err := client.New(cfg, client.Options{Scheme: scheme})
    if err != nil { t.Fatal(err) }

    ns := "research-replacement"
    if e := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); e != nil { t.Fatal(e) }
    claim := &netboxv1.VlanClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "shared", Finalizers: []string{VlanClaimFinalizerName}},
        Spec: netboxv1.VlanClaimSpec{VidRangeStart: 100, VidRangeEnd: 199},
    }
    if e := cl.Create(ctx, claim); e != nil { t.Fatal(e) }
    owned := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "shared"},
        Spec: netboxv1.VlanSpec{Vid: 101},
    }
    if e := controllerutil.SetControllerReference(claim, owned, scheme); e != nil { t.Fatal(e) }
    if e := cl.Create(ctx, owned); e != nil { t.Fatal(e) }

    // Normal API deletion and recreation; no manually populated deletionTimestamp.
    if e := cl.Delete(ctx, owned); e != nil { t.Fatal(e) }
    key := types.NamespacedName{Namespace: ns, Name: "shared"}
    if e := cl.Get(ctx, key, &netboxv1.Vlan{}); !apierrors.IsNotFound(e) {
        t.Fatalf("owned child should be gone before replacement, lookup: %v", e)
    }
    standalone := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "shared"},
        Spec: netboxv1.VlanSpec{Vid: 102},
    }
    if e := cl.Create(ctx, standalone); e != nil { t.Fatal(e) }
    if e := cl.Delete(ctx, claim); e != nil { t.Fatal(e) }
    deleting := &netboxv1.VlanClaim{}
    if e := cl.Get(ctx, key, deleting); e != nil { t.Fatal(e) }
    if deleting.DeletionTimestamp.IsZero() { t.Fatal("claim was not in real Kubernetes deletion state") }

    reconciler := &VlanClaimReconciler{Client: cl, Scheme: scheme}
    if _, e := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key}); e != nil { t.Fatal(e) }
    if e := cl.Get(ctx, key, &netboxv1.Vlan{}); e != nil {
        if apierrors.IsNotFound(e) { t.Fatal("REAL_LIFECYCLE_FOREIGN_VLAN_DELETED: replacement standalone VLAN removed during Claim teardown") }
        t.Fatal(e)
    }
}
