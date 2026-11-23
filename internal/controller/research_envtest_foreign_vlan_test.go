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
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

// Uses a real Kubernetes API Server and etcd, but no external NetBox.
// This is an integration-level control-plane test of a reachable deletion state.
func TestResearchEnvtestForeignVlanSurvivesClaimDeletion(t *testing.T) {
    ctx := context.Background()
    env := &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
        BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
            fmt.Sprintf("1.33.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
    }
    cfg, err := env.Start()
    if err != nil { t.Fatal(err) }
    defer func() { if err := env.Stop(); err != nil { t.Error(err) } }()

    scheme := k8sruntime.NewScheme()
    if err := netboxv1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    if err := corev1.AddToScheme(scheme); err != nil { t.Fatal(err) }
    cl, err := client.New(cfg, client.Options{Scheme: scheme})
    if err != nil { t.Fatal(err) }

    ns := "research-ownership"
    if err := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil { t.Fatal(err) }
    foreign := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "shared-vlan"},
        Spec: netboxv1.VlanSpec{Vid: 101},
    }
    if err := cl.Create(ctx, foreign); err != nil { t.Fatal(err) }

    claim := &netboxv1.VlanClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "shared-vlan", Finalizers: []string{VlanClaimFinalizerName}},
        Spec: netboxv1.VlanClaimSpec{VidRangeStart: 100, VidRangeEnd: 110},
    }
    if err := cl.Create(ctx, claim); err != nil { t.Fatal(err) }
    if err := cl.Delete(ctx, claim); err != nil { t.Fatal(err) }

    key := types.NamespacedName{Namespace: ns, Name: claim.Name}
    deletingClaim := &netboxv1.VlanClaim{}
    if err := cl.Get(ctx, key, deletingClaim); err != nil { t.Fatal(err) }
    if deletingClaim.DeletionTimestamp.IsZero() { t.Fatal("Kubernetes did not preserve the deleting Claim with its finalizer") }

    reconciler := &VlanClaimReconciler{Client: cl, Scheme: scheme}
    if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil { t.Fatal(err) }

    got := &netboxv1.Vlan{}
    if err := cl.Get(ctx, key, got); err != nil {
        if apierrors.IsNotFound(err) {
            t.Fatal("FOREIGN_VLAN_DELETED_REAL_API: Kubernetes API confirms unrelated same-name VLAN was deleted")
        }
        t.Fatal(err)
    }
}
