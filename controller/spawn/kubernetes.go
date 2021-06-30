package spawn

import (
	"io/ioutil"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesSpawner struct {
	// map of regions (kuberentes contexts) to RESTClientGetter
	getters map[string]*genericclioptions.ConfigFlags
}

func (s *KubernetesSpawner) Spawn(d *Daemon) error {
	return nil
}

func (s *KubernetesSpawner) Get(regionid string, daemonid string) *Daemon {
	return nil
}

func (s *KubernetesSpawner) List(region string) []*Daemon {
	return []*Daemon{}
}

func (s *KubernetesSpawner) Regions() []string {
	return []string{}
}

func NewKubernetes() *KubernetesSpawner {
	s := new(KubernetesSpawner)
	// load kubeconfig following the usual kubernetes rules.
	// i.e. obeys KUBECONFIG
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		panic(err)
	}

	// Maybe the config was loaded from some complex chain of several kube configs.
	// simplify things a bit by using a single file.
	kf, err := ioutil.TempFile("/tmp", "dealbot-kubeconfig")
	if err != nil {
		panic(err)
	}
	kf.Close()
	kubefn := kf.Name()

	clientcmd.WriteToFile(rawConfig, kubefn)

	// each context in the kubeconfig represents a dealbot region
	for region := range rawConfig.Contexts {
		getter := genericclioptions.ConfigFlags{
			KubeConfig: &kubefn,
			Context:    &region,
		}
		s.getters[region] = &getter
	}
	return s
}
