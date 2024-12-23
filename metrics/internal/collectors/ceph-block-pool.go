package collectors

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/red-hat-storage/ocs-operator/metrics/v4/internal/options"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	rookclient "github.com/rook/rook/pkg/client/clientset/versioned"
	cephv1listers "github.com/rook/rook/pkg/client/listers/ceph.rook.io/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// component within the project/exporter
	poolMirroringSubsystem = "pool_mirroring"
	defaultRadosNamespace  = "internal"
)

var _ prometheus.Collector = &CephBlockPoolCollector{}

// CephBlockPoolCollector is a custom collector for CephBlockPool Custom Resource
type CephBlockPoolCollector struct {
	MirroringImageHealth *prometheus.Desc
	MirroringStatus      *prometheus.Desc
	Informer             cache.SharedIndexInformer
	InformerRs           cache.SharedIndexInformer
	AllowedNamespaces    []string
}

// NewCephBlockPoolCollector constructs a collector
func NewCephBlockPoolCollector(opts *options.Options) *CephBlockPoolCollector {
	client, err := rookclient.NewForConfig(opts.Kubeconfig)
	if err != nil {
		klog.Error(err)
	}

	lw := cache.NewListWatchFromClient(client.CephV1().RESTClient(), "cephblockpools", searchInNamespace(opts), fields.Everything())
	sharedIndexInformer := cache.NewSharedIndexInformer(lw, &cephv1.CephBlockPool{}, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	lwRs := cache.NewListWatchFromClient(client.CephV1().RESTClient(), "cephblockpoolradosnamespaces", searchInNamespace(opts), fields.Everything())
	sharedIndexInformerRs := cache.NewSharedIndexInformer(lwRs, &cephv1.CephBlockPoolRadosNamespace{}, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	return &CephBlockPoolCollector{
		MirroringImageHealth: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, poolMirroringSubsystem, "image_health"),
			`Pool Mirroring Image Health. 0=OK, 1=UNKNOWN, 2=WARNING & 3=ERROR`,
			[]string{"name", "namespace", "rados_namespace"},
			nil,
		),
		MirroringStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, poolMirroringSubsystem, "status"),
			`Pool Mirroring Status.  0=Disabled, 1=Enabled`,
			[]string{"name", "namespace", "rados_namespace"},
			nil,
		),
		Informer:          sharedIndexInformer,
		InformerRs:        sharedIndexInformerRs,
		AllowedNamespaces: opts.AllowedNamespaces,
	}
}

// Run starts CephBlockPool informer
func (c *CephBlockPoolCollector) Run(stopCh <-chan struct{}) {
	go c.Informer.Run(stopCh)
	go c.InformerRs.Run(stopCh)
}

// Describe implements prometheus.Collector interface
func (c *CephBlockPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ds := []*prometheus.Desc{
		c.MirroringImageHealth,
		c.MirroringStatus,
	}

	for _, d := range ds {
		ch <- d
	}
}

// Collect implements prometheus.Collector interface
func (c *CephBlockPoolCollector) Collect(ch chan<- prometheus.Metric) {
	cephBlockPoolLister := cephv1listers.NewCephBlockPoolLister(c.Informer.GetIndexer())
	cephBlockPools := getAllBlockPools(cephBlockPoolLister, c.AllowedNamespaces)

	if len(cephBlockPools) > 0 {
		c.collectMirroringImageHealth(cephBlockPools, ch)
		c.collectMirroringStatus(cephBlockPools, ch)
	}

	radosNamespaceLister := cephv1listers.NewCephBlockPoolRadosNamespaceLister(c.InformerRs.GetIndexer())
	radosNamespaces := getAllBlockPoolsNamespaces(radosNamespaceLister, c.AllowedNamespaces)
	if len(radosNamespaces) > 0 {
		c.collectMirroringImageHealthRadosNamespace(radosNamespaces, ch)
		c.collectMirroringStatusRadosNamespace(radosNamespaces, ch)
	}
}

func getAllBlockPoolsNamespaces(lister cephv1listers.CephBlockPoolRadosNamespaceLister, namespaces []string) (radosNamespaces []*cephv1.CephBlockPoolRadosNamespace) {
	var tempRadosNamespaces []*cephv1.CephBlockPoolRadosNamespace
	var err error
	if len(namespaces) == 0 {
		radosNamespaces, err = lister.List(labels.Everything())
		if err != nil {
			klog.Errorf("couldn't list CephBlockPools. %v", err)
		}
		return
	}
	for _, namespace := range namespaces {
		tempRadosNamespaces, err = lister.CephBlockPoolRadosNamespaces(namespace).List(labels.Everything())
		if err != nil {
			klog.Errorf("couldn't list CephBlockPool in namespace %s. %v", namespace, err)
			continue
		}
		radosNamespaces = append(radosNamespaces, tempRadosNamespaces...)
	}
	return
}

func getAllBlockPools(lister cephv1listers.CephBlockPoolLister, namespaces []string) (cephBlockPools []*cephv1.CephBlockPool) {
	var tempCephBlockPools []*cephv1.CephBlockPool
	var err error
	if len(namespaces) == 0 {
		cephBlockPools, err = lister.List(labels.Everything())
		if err != nil {
			klog.Errorf("couldn't list CephBlockPools. %v", err)
		}
		return
	}
	for _, namespace := range namespaces {
		tempCephBlockPools, err = lister.CephBlockPools(namespace).List(labels.Everything())
		if err != nil {
			klog.Errorf("couldn't list CephBlockPool in namespace %s. %v", namespace, err)
			continue
		}
		cephBlockPools = append(cephBlockPools, tempCephBlockPools...)
	}
	return
}

func (c *CephBlockPoolCollector) collectMirroringImageHealth(cephBlockPools []*cephv1.CephBlockPool, ch chan<- prometheus.Metric) {
	for _, cephBlockPool := range cephBlockPools {
		var imageHealth string

		if !cephBlockPool.Spec.Mirroring.Enabled {
			continue
		}

		mirroringStatus := cephBlockPool.Status.MirroringStatus
		if mirroringStatus == nil || mirroringStatus.Summary == nil || len(strings.TrimSpace(mirroringStatus.Summary.ImageHealth)) == 0 {
			klog.Errorf("Mirroring is enabled on CephBlockPool %q but image health status is not available.", cephBlockPool.Name)
			continue
		}

		switch mirroringStatus.Summary.ImageHealth {
		case "OK":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 0,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		case "UNKNOWN":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 1,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		case "WARNING":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 2,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		case "ERROR":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 3,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		default:
			klog.Errorf("Invalid image health, %q, for pool %s. Must be OK, UNKNOWN, WARNING or ERROR.", imageHealth, cephBlockPool.Name)
		}
	}
}

func (c *CephBlockPoolCollector) collectMirroringStatus(cephBlockPools []*cephv1.CephBlockPool, ch chan<- prometheus.Metric) {
	for _, cephBlockPool := range cephBlockPools {
		switch cephBlockPool.Spec.Mirroring.Enabled {
		case true:
			ch <- prometheus.MustNewConstMetric(c.MirroringStatus,
				prometheus.GaugeValue, 1,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		case false:
			ch <- prometheus.MustNewConstMetric(c.MirroringStatus,
				prometheus.GaugeValue, 0,
				cephBlockPool.Name,
				cephBlockPool.Namespace, defaultRadosNamespace)
		default:
			klog.Errorf("Invalid spec for pool %s. CephBlockPool.Spec.Mirroring.Enabled must be true or false", cephBlockPool.Name)
		}
	}
}

func (c *CephBlockPoolCollector) collectMirroringImageHealthRadosNamespace(radosNamespace []*cephv1.CephBlockPoolRadosNamespace, ch chan<- prometheus.Metric) {
	for _, radosNamespace := range radosNamespace {
		var imageHealth string

		if !(radosNamespace.Spec.Mirroring.Mode == cephv1.RadosNamespaceMirroringModePool || radosNamespace.Spec.Mirroring.Mode == cephv1.RadosNamespaceMirroringModeImage) {
			continue
		}

		mirroringStatus := radosNamespace.Status.MirroringStatus
		if mirroringStatus == nil || mirroringStatus.Summary == nil || len(strings.TrimSpace(mirroringStatus.Summary.ImageHealth)) == 0 {
			klog.Errorf("Mirroring is enabled on CephBlockPoolRadosNamespace %q but image health status is not available.", radosNamespace.Name)
			continue
		}

		switch mirroringStatus.Summary.ImageHealth {
		case "OK":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 0,
				radosNamespace.Spec.BlockPoolName,
				radosNamespace.Namespace, radosNamespace.Name)
		case "UNKNOWN":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 1,
				radosNamespace.Spec.BlockPoolName,
				radosNamespace.Namespace, radosNamespace.Name)
		case "WARNING":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 2,
				radosNamespace.Spec.BlockPoolName,
				radosNamespace.Namespace, radosNamespace.Name)
		case "ERROR":
			ch <- prometheus.MustNewConstMetric(c.MirroringImageHealth,
				prometheus.GaugeValue, 3,
				radosNamespace.Spec.BlockPoolName,
				radosNamespace.Namespace, radosNamespace.Name)
		default:
			klog.Errorf("Invalid image health, %q, for rados namespace %s in pool %s. Must be OK, UNKNOWN, WARNING or ERROR.", imageHealth, radosNamespace.Name, radosNamespace.Spec.BlockPoolName)
		}
	}
}

func (c *CephBlockPoolCollector) collectMirroringStatusRadosNamespace(radosNamespace []*cephv1.CephBlockPoolRadosNamespace, ch chan<- prometheus.Metric) {
	for _, cephBlockPool := range radosNamespace {
		switch cephBlockPool.Spec.Mirroring.Mode {
		case cephv1.RadosNamespaceMirroringModePool:
			fallthrough
		case cephv1.RadosNamespaceMirroringModeImage:
			ch <- prometheus.MustNewConstMetric(c.MirroringStatus,
				prometheus.GaugeValue, 1,
				cephBlockPool.Spec.BlockPoolName,
				cephBlockPool.Namespace, cephBlockPool.Name)
		default:
			ch <- prometheus.MustNewConstMetric(c.MirroringStatus,
				prometheus.GaugeValue, 0,
				cephBlockPool.Spec.BlockPoolName,
				cephBlockPool.Namespace, cephBlockPool.Name)
		}
	}
}
