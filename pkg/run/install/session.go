package install

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/mitchellh/go-ps"
	"github.com/weaveworks/weave-gitops/pkg/logger"
	"github.com/weaveworks/weave-gitops/pkg/run/session/connect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Session struct {
	name                    string
	namespace               string
	fluxNamespace           string
	kubeClient              client.Client
	log                     logger.Logger
	dashboardHashedPassword string
	skipDashboardInstall    bool
	portForwards            []string
	automationKind          string
}

func (s *Session) Start() error {
	if err := installVCluster(s.kubeClient, s.name, s.namespace, s.fluxNamespace, s.portForwards, s.automationKind); err != nil {
		return err
	}

	return nil
}

func (s *Session) Connect() error {
	subProcArgs := append(os.Args,
		// we must run the sub-process without a session.
		"--no-session",
		// we must let the sub-run know that this is the session name of the sub-process
		"--x-session-name", s.name,
		// vclusters are always new clusters, that doesn't mean we haven't bootstrapped the outer cluster.
		"--no-bootstrap",
		// allow the sub-process to connect to the vcluster context.
		"--allow-k8s-context="+s.name,
		// we must skip resource cleanup in the sub-process because we are already deleting the vcluster.
		// it's for optimization purposes.
		"--skip-resource-cleanup",
	)

	if s.skipDashboardInstall {
		// we skip dashboard install in the sub-process.
		subProcArgs = append(subProcArgs, "--skip-dashboard-install")
	} else if s.dashboardHashedPassword != "" {
		// we forward dashboard password from host to session too.
		subProcArgs = append(subProcArgs, "--dashboard-hashed-password="+s.dashboardHashedPassword)
	}

	// we support statefulset pod only for now.
	conn := &connect.Connection{
		PodName:               s.name + "-0",
		Log:                   s.log,
		Namespace:             s.namespace,
		BackgroundProxy:       false,
		KubeConfigContextName: s.name,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-c
		signal.Reset(sig)

		thisProc := os.Getpid()
		allProcesses, err := ps.Processes()

		if err != nil {
			return
		}

		for _, proc := range allProcesses {
			if proc.PPid() == thisProc {
				// ok it's a child process, obtain the process object
				procObject, err := os.FindProcess(proc.Pid())
				if err != nil {
					continue
				}

				// and notify it
				if err := procObject.Signal(syscall.SIGUSR1); err != nil {
					return
				}
			}
		}
	}()

	err := conn.Connect(s.name, subProcArgs)

	return err
}

func (s *Session) Close() error {
	if err := uninstallVcluster(s.kubeClient, s.name, s.namespace); err != nil {
		return err
	}

	return nil
}

func NewSession(log logger.Logger,
	kubeClient client.Client,
	name string, namespace string,
	fluxNamespace string, portForwards []string,
	skipDashboardInstall bool, dashboardHashedPassword string,
	automationKind string) (*Session, error) {
	return &Session{
		name:                    name,
		namespace:               namespace,
		fluxNamespace:           fluxNamespace,
		kubeClient:              kubeClient,
		log:                     log,
		portForwards:            portForwards,
		skipDashboardInstall:    skipDashboardInstall,
		dashboardHashedPassword: dashboardHashedPassword,
		automationKind:          automationKind,
	}, nil
}
