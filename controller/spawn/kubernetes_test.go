package spawn

import (
	"errors"
	"net/http"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	k8sfake "k8s.io/client-go/rest/fake"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	k8stesting "k8s.io/kubectl/pkg/cmd/testing"
)

// inspiration for the mock k8s client:
// https://github.com/helm/helm/blob/main/pkg/kube/client_test.go#L114
func newTestKubernetes(t *testing.T, reqs chan *http.Request) *KubernetesSpawner {
	t.Helper()
	s := new(KubernetesSpawner)
	factory := k8stesting.NewTestFactory().WithNamespace("fake")
	factory.UnstructuredClient = &k8sfake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: k8sfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			// TODO: write mocks
			reqs <- req
			return &http.Response{}, nil
		}),
	}
	s.getters = make(map[string]genericclioptions.RESTClientGetter)
	s.getters["fake"] = factory
	c := clientcmdapi.NewContext()
	c.Namespace = "fake"
	s.rawConfig = clientcmdapi.Config{
		Contexts: map[string]*clientcmdapi.Context{
			"fake": c,
		},
	}
	return s
}

// tests that the spawn request would be sent to kubernetes
func TestKubernetesSpawnerSpawn(t *testing.T) {
	reqs := make(chan *http.Request)
	errs := make(chan error)
	go func(reqs chan *http.Request) {
		var count int
		for range reqs {
			count++
		}
		if count != 1 {
			errs <- errors.New("incorrect number of requests to kubernetes")
		}
		close(errs)
	}(reqs)
	s := newTestKubernetes(t, reqs)
	d := randomDaemon()
	d.Region = "fake"
	err := s.Spawn(d)
	if err != nil {
		t.Fatal(err)
	}
	close(reqs)
	for err := range errs {
		t.Fatal(err)
	}
}

// TODO:
// With a mocked response, test that the daemons being returend are correct.
func TestKubernetesSpawnerList(t *testing.T) {
}
