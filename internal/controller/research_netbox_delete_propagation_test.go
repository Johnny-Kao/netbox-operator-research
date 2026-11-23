package controller

import (
    "context"
    "fmt"
    "net/http"
    "path/filepath"
    "runtime"
    "testing"

    netboxv1 "github.com/netbox-community/netbox-operator/api/v1"
    "github.com/netbox-community/netbox-operator/gen/mock_interfaces"
    "github.com/netbox-community/netbox-operator/pkg/netbox/api"
    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    k8sruntime "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/tools/record"
    "go.uber.org/mock/gomock"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

// The downstream consequence of deleting a foreign Vlan with an active
// operator finalizer and a real NetBox record ID. No external NetBox instance
// is contacted: a mock records whether the production DeleteVlan path issues
// the NetBox REST DELETE for that ID.
func TestResearchForeignVlanNetboxDeletePropagation(t *testing.T) {
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
    ns := "research-netbox-delete"
    if e := cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); e != nil { t.Fatal(e) }
    key := types.NamespacedName{Namespace: ns, Name: "independent-vlan"}

    const netboxID int32 = 4242
    foreign := &netboxv1.Vlan{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: ns, Name: key.Name,
            Finalizers: []string{VlanFinalizerName},
        },
        Spec: netboxv1.VlanSpec{Vid: 104, PreserveInNetbox: false},
    }
    if e := cl.Create(ctx, foreign); e != nil { t.Fatal(e) }
    foreign.Status.VlanId = int64(netboxID)
    if e := cl.Status().Update(ctx, foreign); e != nil { t.Fatal(e) }

    claim := &netboxv1.VlanClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: key.Name, Finalizers: []string{VlanClaimFinalizerName}},
        Spec: netboxv1.VlanClaimSpec{VidRangeStart: 100, VidRangeEnd: 150},
    }
    if e := cl.Create(ctx, claim); e != nil { t.Fatal(e) }
    if e := cl.Delete(ctx, claim); e != nil { t.Fatal(e) }

    claimR := &VlanClaimReconciler{Client: cl, Scheme: scheme}
    if _, e := claimR.Reconcile(ctx, ctrl.Request{NamespacedName: key}); e != nil { t.Fatal(e) }

    current := &netboxv1.Vlan{}
    err = cl.Get(ctx, key, current)
    if apierrors.IsNotFound(err) {
        t.Fatal("foreign VLAN disappeared before NetBox finalizer could execute")
    }
    if err != nil { t.Fatal(err) }

    mockCtrl := gomock.NewController(t)
    mockAPI := mock_interfaces.NewMockIpamAPI(mockCtrl)
    mockDelete := mock_interfaces.NewMockIpamVlansDestroyRequest(mockCtrl)
    netboxDeletes := 0
    mockAPI.EXPECT().IpamVlansDestroy(gomock.Any(), netboxID).DoAndReturn(func(_ context.Context, _ int32) interface{} {
        netboxDeletes++
        return mockDelete
    }).AnyTimes()
    mockDelete.EXPECT().Execute().Return(&http.Response{StatusCode: 204, Body: http.NoBody}, nil).AnyTimes()

    if !current.DeletionTimestamp.IsZero() {
        vlanR := &VlanReconciler{
            Client: cl, Scheme: scheme,
            NetboxClient: api.NewNetboxCompositeClient(&api.NetboxClientV3{}, &api.NetboxClientV4{IpamAPI: mockAPI}),
            EventStatusRecorder: NewEventStatusRecorder(record.NewFakeRecorder(20)),
        }
        if _, e := vlanR.Reconcile(ctx, ctrl.Request{NamespacedName: key}); e != nil { t.Fatal(e) }
    }

    if netboxDeletes != 0 {
        t.Fatalf("NETBOX_FOREIGN_VLAN_DELETE_CALLED: VLAN Controller issued %d DELETE request(s) for unrelated NetBox VLAN ID %d", netboxDeletes, netboxID)
    }

    // The guarded Claim path must keep the standalone, already assigned VLAN.
    surviving := &netboxv1.Vlan{}
    if e := cl.Get(ctx, key, surviving); e != nil { t.Fatalf("foreign VLAN is missing after claim teardown: %v", e) }
    if !surviving.DeletionTimestamp.IsZero() { t.Fatal("foreign VLAN was put in deletion state") }
}
