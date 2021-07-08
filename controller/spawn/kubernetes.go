package spawn

import (
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
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
	client.Deployed = true
	releases, err := client.Run()
	if err != nil {
		log.Infow("failed to list releases", "err", err)
	}
	for _, r := range releases {
		d := daemonFromRelease(r, regionid)
		daemons = append(daemons, d)
	}
	return daemons, nil
}

func (s *KubernetesSpawner) Regions() []string {
	var regions []string
	for region := range s.getters {
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

// TODO
// horrible. Just horrible.
// config is map[string]interface{}, recursive
// but we know that config["application"]["container"]
// is a kubernetes container, and that configuration
// is visible there in environment variables.
// If the environment variables exist, use them to create
// the Daemon struct. If they don't exist, the Daemon will
// have zero-values
func daemonFromRelease(r *release.Release, regionid string) (daemon *Daemon) {
	container := r.Config["application"].(map[string]interface{})["container"]
	kcontainer := new(corev1.Container)
	buf, _ := yaml.Marshal(container)
	yaml.Unmarshal(buf, kcontainer)
	var minfil, mincap int
	var tags []string
	wallet := new(Wallet)
	for _, env := range kcontainer.Env {
		switch env.Name {
		case "DEALBOT_TAGS":
			tags = strings.Split(env.Value, ",")
		case "DEALBOT_WALLET_ADDRESS":
			wallet.Address = env.Value
		case "DEALBOT_MIN_FIL":
			minfil, _ = strconv.Atoi(env.Value)
		case "DEALBOT_MIN_CAP":
			mincap, _ = strconv.Atoi(env.Value)
		}
	}
	return &Daemon{
		Id:     r.Name,
		Region: regionid,
		Tags:   tags,
		Wallet: wallet,
		MinFil: minfil,
		MinCap: mincap,
	}
}
