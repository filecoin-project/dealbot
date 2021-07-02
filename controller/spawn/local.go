package spawn

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Start dealbot daemons locally
type LocalSpawner struct {
	sync.Mutex
	endpoint string
	cmds     map[string]exec.Cmd
}

func (s *LocalSpawner) Spawn(d *Daemon) error {
	log.Infow("spawining daemon", "region", d.Region)
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data_dir, err := ioutil.TempDir("/tmp", d.Id)
	if err != nil {
		return err
	}
	cmd := exec.Cmd{
		Path: exe,
		Env: envStr(map[string]string{
			"DEALBOT_ID":                  d.Id,
			"DEALBOT_DATA_DIRECTORY":      data_dir,
			"DEALBOT_TAGS":                strings.Join(d.Tags, ","),
			"DEALBOT_WALLET_ADDRESS":      d.Wallet.Address,
			"DEALBOT_WORKERS":             strconv.Itoa(d.Workers),
			"DEALBOT_CONTROLLER_ENDPOINT": s.endpoint,
			"DEALBOT_MIN_FIL":             strconv.Itoa(d.MinFil),
			"DEALBOT_MIN_CAP":             strconv.Itoa(d.MinCap),
		}),
		Args:   []string{exe, "daemon"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	log.Infow("starting daemon with cli", "command", cmd.String())
	err = cmd.Start()
	if err != nil {
		log.Info("failed to start daemon", "id", d.Id, "err", err)
		return err
	}
	s.Lock()
	s.cmds[d.Id] = cmd
	s.Unlock()
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Errorw("dealbot daemon crashed", "id", d.Id, "err", err)
		}
		log.Info("dealbot exited normally", "id", d.Id, "code", cmd.ProcessState.ExitCode())
		s.Lock()
		delete(s.cmds, d.Id)
		s.Unlock()
	}()
	return nil
}

func (s *LocalSpawner) Get(regionid string, daemonid string) (*Daemon, error) {
	s.Lock()
	defer s.Unlock()
	daemoncmd, ok := s.cmds[daemonid]
	if !ok {
		return nil, DaemonNotFound
	}
	return daemonFromCmd(daemoncmd, daemonid), nil
}

func (s *LocalSpawner) List(regionid string) ([]*Daemon, error) {
	s.Lock()
	defer s.Unlock()
	daemons := make([]*Daemon, len(s.cmds))
	var idx int
	for daemonid, cmd := range s.cmds {
		daemon := daemonFromCmd(cmd, daemonid)
		daemons[idx] = daemon
		idx += 1
	}
	return daemons, nil
}

func (s *LocalSpawner) Regions() []string {
	return []string{"local"}
}

func NewLocal(endpoint string) (s *LocalSpawner) {
	s = &LocalSpawner{
		endpoint: endpoint,
		cmds:     make(map[string]exec.Cmd),
	}
	return s
}

// exec.Cmd.Env is []string fomatted like this
// ENVVAR=VALUE
// convert to map for usability
func envMap(env []string) map[string]string {
	em := make(map[string]string)
	for _, e := range env {
		sp := strings.Split(e, "=")
		em[sp[0]] = sp[1]
	}
	return em
}

// convert map[string]string enviromnment into []string
func envStr(em map[string]string) []string {
	env := os.Environ()
	for k, v := range em {
		env = append(env, strings.Join([]string{k, v}, "="))
	}
	return env
}

func daemonFromCmd(daemoncmd exec.Cmd, daemonid string) *Daemon {
	env := envMap(daemoncmd.Env)
	workers, _ := strconv.Atoi(env["DEALBOT_WORKERS"])
	minfil, _ := strconv.Atoi(env["DEALBOT_MIN_FIL"])
	mincap, _ := strconv.Atoi(env["DEALBOT_MIN_CAP"])
	return &Daemon{
		Id:     daemonid,
		Region: "local",
		Tags:   strings.Split(env["DEALBOT_TAGS"], ","),
		Wallet: &Wallet{
			Address: env["DEALBOT_WALLET_ADDRESS"],
		},
		Workers: workers,
		MinFil:  minfil,
		MinCap:  mincap,
	}
}
