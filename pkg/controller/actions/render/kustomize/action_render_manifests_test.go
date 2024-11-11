package kustomize_test

import (
	"context"
	"path"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/xid"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	mk "github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

	. "github.com/onsi/gomega"
)

const testRenderResourcesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
resources:
- test-resources-cm.yaml
- test-resources-deployment-managed.yaml
- test-resources-deployment-unmanaged.yaml
- test-resources-deployment-forced.yaml
`

const testRenderResourcesConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  foo: bar
`

const testRenderResourcesManaged = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-managed
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        resources:
            limits:
              memory: 200Mi
              cpu: 1
            requests:
              memory: 100Mi
              cpu: 100m
`

const testRenderResourcesUnmanaged = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-unmanaged
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        resources:
            limits:
              memory: 200Mi
              cpu: 1
            requests:
              memory: 100Mi
              cpu: 100m
`
const testRenderResourcesForced = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-forced
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
`

func TestRenderResourcesAction(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	ns := xid.New().String()
	id := xid.New().String()
	fs := filesys.MakeFsInMemory()

	_ = fs.MkdirAll(path.Join(id, mk.DefaultKustomizationFilePath))
	_ = fs.WriteFile(path.Join(id, mk.DefaultKustomizationFileName), []byte(testRenderResourcesKustomization))
	_ = fs.WriteFile(path.Join(id, "test-resources-cm.yaml"), []byte(testRenderResourcesConfigMap))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-managed.yaml"), []byte(testRenderResourcesManaged))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-unmanaged.yaml"), []byte(testRenderResourcesUnmanaged))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-forced.yaml"), []byte(testRenderResourcesForced))

	cl, err := fakeclient.New(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	action := kustomize.NewAction(
		kustomize.WithLabel("component.opendatahub.io/name", "foo"),
		kustomize.WithLabel("platform.opendatahub.io/namespace", ns),
		kustomize.WithAnnotation("platform.opendatahub.io/release", "1.2.3"),
		kustomize.WithAnnotation("platform.opendatahub.io/type", "managed"),
		// for testing
		kustomize.WithManifestsOptions(
			mk.WithEngineFS(fs),
		),
	)

	rr := types.ReconciliationRequest{
		Client:    cl,
		Instance:  &componentsv1.Dashboard{},
		DSCI:      &dsciv1.DSCInitialization{Spec: dsciv1.DSCInitializationSpec{ApplicationsNamespace: ns}},
		DSC:       &dscv1.DataScienceCluster{},
		Release:   cluster.Release{Name: cluster.OpenDataHub},
		Manifests: []types.ManifestInfo{{Path: id}},
	}

	err = action(ctx, &rr)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(rr.Resources).Should(And(
		HaveLen(4),
		HaveEach(And(
			jq.Match(`.metadata.namespace == "%s"`, ns),
			jq.Match(`.metadata.labels."component.opendatahub.io/name" == "%s"`, "foo"),
			jq.Match(`.metadata.labels."platform.opendatahub.io/namespace" == "%s"`, ns),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/release" == "%s"`, "1.2.3"),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/type" == "%s"`, "managed"),
		)),
	))
}

const testRenderResourcesWithCacheKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
resources:
- test-resources-deployment.yaml
`

const testRenderResourcesWithCacheDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-managed
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        resources:
            limits:
              memory: 200Mi
              cpu: 1
            requests:
              memory: 100Mi
              cpu: 100m
`

func TestRenderResourcesWithCacheAction(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	ns := xid.New().String()
	id := xid.New().String()
	fs := filesys.MakeFsInMemory()

	_ = fs.MkdirAll(path.Join(id, mk.DefaultKustomizationFilePath))
	_ = fs.WriteFile(path.Join(id, mk.DefaultKustomizationFileName), []byte(testRenderResourcesWithCacheKustomization))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment.yaml"), []byte(testRenderResourcesWithCacheDeployment))

	cl, err := fakeclient.New(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	action := kustomize.NewAction(
		kustomize.WithCache(kustomize.DefaultCachingKeyFn),
		kustomize.WithLabel(labels.ComponentPartOf, "foo"),
		kustomize.WithLabel("platform.opendatahub.io/namespace", ns),
		kustomize.WithAnnotation("platform.opendatahub.io/release", "1.2.3"),
		kustomize.WithAnnotation("platform.opendatahub.io/type", "managed"),
		// for testing
		kustomize.WithManifestsOptions(
			mk.WithEngineFS(fs),
		),
	)

	render.RenderedResourcesTotal.Reset()

	for i := int64(0); i < 3; i++ {
		d := componentsv1.Dashboard{}

		if i >= 1 {
			d.Generation = 1
		}

		rr := types.ReconciliationRequest{
			Client:    cl,
			Instance:  &d,
			DSCI:      &dsciv1.DSCInitialization{Spec: dsciv1.DSCInitializationSpec{ApplicationsNamespace: ns}},
			DSC:       &dscv1.DataScienceCluster{},
			Release:   cluster.Release{Name: cluster.OpenDataHub},
			Manifests: []types.ManifestInfo{{Path: id}},
		}

		err = action(ctx, &rr)

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(rr.Resources).Should(And(
			HaveLen(1),
			HaveEach(And(
				jq.Match(`.metadata.namespace == "%s"`, ns),
				jq.Match(`.metadata.labels."%s" == "%s"`, labels.ComponentPartOf, "foo"),
				jq.Match(`.metadata.labels."platform.opendatahub.io/namespace" == "%s"`, ns),
				jq.Match(`.metadata.annotations."platform.opendatahub.io/release" == "%s"`, "1.2.3"),
				jq.Match(`.metadata.annotations."platform.opendatahub.io/type" == "%s"`, "managed"),
			)),
		))

		rc := testutil.ToFloat64(render.RenderedResourcesTotal)

		switch i {
		case 0:
			g.Expect(rc).Should(BeNumerically("==", 1))
		case 1:
			g.Expect(rc).Should(BeNumerically("==", 2))
		case 2:
			g.Expect(rc).Should(BeNumerically("==", 2))
		}
	}
}