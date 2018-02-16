package crd

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiexts "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/solo-io/glue-storage"
	crdclientset "github.com/solo-io/glue-storage/crd/client/clientset/versioned"
	crdv1 "github.com/solo-io/glue-storage/crd/solo.io/v1"
	"github.com/solo-io/glue/pkg/log"
)

type Client struct {
	v1 *v1client
}

func NewStorage(cfg *rest.Config, namespace string, syncFrequency time.Duration) (storage.Interface, error) {
	if namespace == "" {
		namespace = GlueDefaultNamespace
	}
	crdClient, err := crdclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	apiextClient, err := apiexts.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		v1: &v1client{
			upstreams: &upstreamsClient{
				crds:          crdClient,
				namespace:     namespace,
				syncFrequency: syncFrequency,
			},
			virtualHosts: &virtualHostsClient{
				crds:          crdClient,
				namespace:     namespace,
				syncFrequency: syncFrequency,
			},
			apiexts:    apiextClient,
			kubeclient: kubeClient,
			namespace:  namespace,
		},
	}, nil
}

func (c *Client) V1() storage.V1 {
	return c.v1
}

type v1client struct {
	apiexts      apiexts.Interface
	kubeclient   kubernetes.Interface
	upstreams    *upstreamsClient
	virtualHosts *virtualHostsClient
	namespace    string
}

func (c *v1client) Register() error {
	// create namespace if it doesnt exist
	if _, err := c.kubeclient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.namespace,
		},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %v: %v", c.namespace, err)
	}

	for _, crd := range crdv1.KnownCRDs {
		toRegister := &v1beta1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.FullName()},
			Spec: v1beta1.CustomResourceDefinitionSpec{
				Group:   crd.Group,
				Version: crd.Version,
				Scope:   v1beta1.NamespaceScoped,
				Names: v1beta1.CustomResourceDefinitionNames{
					Plural: crd.Plural,
					Kind:   crd.Kind,
				},
			},
		}
		log.Debugf("registering crd %v", crd)
		if _, err := c.apiexts.ApiextensionsV1beta1().CustomResourceDefinitions().Create(toRegister); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create crd: %v", err)
		}
	}
	return nil
}

func (c *v1client) Upstreams() storage.Upstreams {
	return c.upstreams
}

func (c *v1client) VirtualHosts() storage.VirtualHosts {
	return c.virtualHosts
}
