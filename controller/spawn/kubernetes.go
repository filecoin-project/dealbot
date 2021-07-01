package spawn

import (
	"io/ioutil"
	"path"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type KubernetesSpawner struct {
	// map of regions (kuberentes contexts) to RESTClientGetter
	getters   map[string]*genericclioptions.ConfigFlags
	rawConfig clientcmdapi.Config
}

func (s *KubernetesSpawner) Spawn(d *Daemon) error {
	actionConfig, err := s.actionConfig(d.Region)
	if err != nil {
		log.Infow("could not create actionConfig during Spawn", "err", err)
		return err
	}
	client := action.NewInstall(actionConfig)
	chartPath, err := client.ChartPathOptions.LocateChart("filecoin/lotus-bundle",
		helmcli.New())
	if err != nil {
		log.Infow("could not determine chart path", "err", err)
		return err
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		log.Infow("could not load chart", "chartPath", chartPath, "err", err)
		return err
	}
	client.ReleaseName = d.Id
	client.UseReleaseName = true
	client.CreateNamespace = false

	vals, err := dealbotValues(d)
	if err != nil {
		return err
	}
	_, err = client.Run(chart, vals)
	if err != nil {
		log.Info("could not install chart", "err", err, "release", client.ReleaseName)
		return err
	}

	return nil
}

func (s *KubernetesSpawner) Get(regionid string, daemonid string) (daemon *Daemon, err error) {
	actionConfig, err := s.actionConfig(regionid)
	if err != nil {
		log.Infow("could not create actionConfig during daemon Get", "err", err)
		return daemon, err
	}
	client := action.NewGet(actionConfig)
	r, err := client.Run(daemonid)
	if err != nil {
		return daemon, DaemonNotFound
	}
	daemon = daemonFromRelease(r, regionid)
	return daemon, nil
}

func (s *KubernetesSpawner) List(regionid string) (daemons []*Daemon, err error) {
	actionConfig, err := s.actionConfig(regionid)
	if err != nil {
		log.Infow("could not create actionConfig during daemon List", "err", err)
		return daemons, err
	}
	client := action.NewList(actionConfig)
	client.All = true
	// client.Deployed = true
	releases, err := client.Run()
	log.Info("how many releases?", len(releases))
	if err != nil {
		log.Infow("failed to list releases", "err", err)
	}
	// there could be helm installations other than dealbot daemons.
	for _, r := range releases {
		// if r.Labels["DEALBOT_CONTROLLER_MANAGED"] == "true" {
		log.Info("release name outer", r.Name)
		d := daemonFromRelease(r, regionid)
		daemons = append(daemons, d)
		// }
	}
	return daemons, nil
}

func (s *KubernetesSpawner) Regions() []string {
	var regions []string
	for region, getter := range s.getters {
		log.Info("getter", getter)
		regions = append(regions, region)
	}
	return regions
}

func (s *KubernetesSpawner) actionConfig(regionid string) (*action.Configuration, error) {
	getter, ok := s.getters[regionid]
	if !ok {
		return nil, RegionNotFound
	}
	c := s.rawConfig.Contexts[regionid]
	actionConfig := new(action.Configuration)
	actionConfig.Init(getter, c.Namespace, "", log.Debugw)
	return actionConfig, actionConfig.KubeClient.IsReachable()
}

func NewKubernetes() *KubernetesSpawner {
	s := new(KubernetesSpawner)
	s.getters = make(map[string]*genericclioptions.ConfigFlags)
	// load kubeconfig following the usual kubernetes rules.
	// i.e. obeys KUBECONFIG
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides)
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		panic(err)
	}
	s.rawConfig = rawConfig

	// Maybe the config was loaded from some complex chain of several kube configs.
	// simplify things a bit by flattening the config into a single file.
	tmpdir, err := ioutil.TempDir("/tmp", "dealbot-controller")
	kubefn := path.Join(tmpdir, "kubeconfig")
	if err := clientcmd.WriteToFile(rawConfig, kubefn); err != nil {
		panic(err)
	}

	// each context in the kubeconfig represents a dealbot region
	// only add contexts that have a namespace defined, that way no
	// additional configuration is requried.
	for region, c := range rawConfig.Contexts {
		maskRegion := region
		if c.Namespace == "" {
			log.Warnw("not adding context without namespace", "context", region)
			continue
		}
		getter := genericclioptions.ConfigFlags{
			KubeConfig: &kubefn,
			Context:    &maskRegion,
			Namespace:  &c.Namespace,
		}
		s.getters[region] = &getter
	}
	return s
}

func daemonFromRelease(r *release.Release, regionid string) (daemon *Daemon) {
	var minfil, mincap int
	daemon = &Daemon{
		Id:     r.Name,
		Region: regionid,
		Tags:   strings.Split(r.Labels["DEALBOT_TAGS"], ","),
		Wallet: &Wallet{
			Address: r.Labels["DEALBOT_WALLET"],
		},
		MinFil: minfil,
		MinCap: mincap,
	}
	return daemon
}
