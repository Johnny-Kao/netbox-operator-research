package controller

import (
    "context"
    "fmt"
    "path/filepath"
    "runtime"
    "testing"
    "time"

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
    metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// A real controller-runtime Manager is started AFTER the operator downtime
// sequence. No explicit Reconcile call is made: controller startup / cache
// initial listing must automatically process the queued deleting Claim.
func TestResearchManagerRestartAfterVlanReplacement(t *testing.T) {
    ctx := context.Background()
    env := &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
        BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s", fmt.Sprintf("1.33.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
    }
    cfg, err := env.Start()
    if err != nil { t.Fatal(err) }
    defer func() { if e := env.Stop(); e != nil { t.Error(e) } }()

    scheme := k8sruntime.NewScheme()
    if e := netboxv1.AddToScheme(scheme); e != nil { t.Fatal(e) }
    if e := corev1.AddToScheme(scheme); e != nil { t.Fatal(e) }
    cl, err := client.New(cfg, client.Options{Scheme: scheme})
    if err != nil { t.Fatal(err) }
    ns := "research-manager-restart"
    if e := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); e != nil { t.Fatal(e) }
    claim := &netboxv1.VlanClaim{
        ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: ns, Finalizers: []string{VlanClaimFinalizerName}},
        Spec: netboxv1.VlanClaimSpec{VidRangeStart: 100, VidRangeEnd: 199},
    }
    if e := cl.Create(ctx, claim); e != nil { t.Fatal(e) }
    owned := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: ns},
        Spec: netboxv1.VlanSpec{Vid: 100},
    }
    if e := controllerutil.SetControllerReference(claim, owned, scheme); e != nil { t.Fatal(e) }
    if e := cl.Create(ctx, owned); e != nil { t.Fatal(e) }
    if e := cl.Delete(ctx, owned); e != nil { t.Fatal(e) }

    key := types.NamespacedName{Namespace: ns, Name: claim.Name}
    if e := cl.Get(ctx, key, &netboxv1.Vlan{}); !apierrors.IsNotFound(e) {
        t.Fatalf("original owned child should be gone: %v", e)
    }
    standalone := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: claim.Name, Finalizers: []string{VlanFinalizerName}},
        Spec: netboxv1.VlanSpec{Vid: 102, PreserveInNetbox: false},
    }
    if e := cl.Create(ctx, standalone); e != nil { t.Fatal(e) }
    if e := cl.Delete(ctx, claim); e != nil { t.Fatal(e) }

    // Manager starts now, as after a Kubernetes operator restart.
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        Scheme: scheme,
        Metrics: metricsserver.Options{BindAddress: "0"},
        HealthProbeBindAddress: "0",
    })
    if err != nil { t.Fatal(err) }
    r := &VlanClaimReconciler{Client: mgr.GetClient(), Scheme: scheme}
    if e := r.SetupWithManager(mgr); e != nil { t.Fatal(e) }
    runCtx, cancel := context.WithCancel(ctx)
    stopped := make(chan error, 1)
    go func() { stopped <- mgr.Start(runCtx) }()
    defer func() { cancel(); <-stopped }()

    deadline := time.Now().Add(18 * time.Second)
    for time.Now().Before(deadline) {
        got := &netboxv1.Vlan{}
        e := cl.Get(ctx, key, got)
        if e == nil && !got.DeletionTimestamp.IsZero() {
            t.Fatal("MANAGER_RESTART_FOREIGN_VLAN_DELETED: live controller-runtime manager marked standalone VLAN for deletion after restart")
        }
        if apierrors.IsNotFound(e) {
            t.Fatal("MANAGER_RESTART_FOREIGN_VLAN_DELETED: live controller-runtime manager removed standalone VLAN after restart")
        }
        if e != nil { t.Fatal(e) }

        left := &netboxv1.VlanClaim{}
        e = cl.Get(ctx, key, left)
        if apierrors.IsNotFound(e) {
            // A safe implementation removes the stale Claim finalizer.
            if got.DeletionTimestamp.IsZero() { return }
        } else if e != nil {
            t.Fatal(e)
        }
        time.Sleep(150 * time.Millisecond)
    }
    t.Fatal("MANAGER_RESTART_NO_PROGRESS: stale Claim finalizer was not cleared within the deadline")
}
