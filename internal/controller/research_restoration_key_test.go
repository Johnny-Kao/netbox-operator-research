package controller

import (
    "testing"

    netboxv1 "github.com/netbox-community/netbox-operator/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Research only: this test intentionally fails on the unchanged upstream baseline.
// Both identities are valid Kubernetes namespace/name pairs, but concatenating
// their fields without boundaries yields the same restoration hash.
func TestResearchIpAddressRestorationKeyUniqueAcrossNamespaces(t *testing.T) {
    first := &netboxv1.IpAddressClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "ab", Name: "c"},
        Spec: netboxv1.IpAddressClaimSpec{ParentPrefix: "10.10.0.0/24"},
    }
    second := &netboxv1.IpAddressClaim{
        ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "bc"},
        Spec: netboxv1.IpAddressClaimSpec{ParentPrefix: "10.10.0.0/24"},
    }
    firstHash := generateIpAddressRestorationHash(first)
    secondHash := generateIpAddressRestorationHash(second)
    if firstHash == secondHash {
        t.Fatalf("RESTORATION_KEY_COLLISION: %s/%s and %s/%s share restoration hash %s",
            first.Namespace, first.Name, second.Namespace, second.Name, firstHash)
    }
}
