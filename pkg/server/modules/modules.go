package modules

import (
	"context"

	"github.com/grafana/dskit/modules"
	"github.com/grafana/dskit/services"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/server/backgroundsvcs"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/grpcserver"
	"github.com/grafana/grafana/pkg/services/store/kind"
	objectdummyserver "github.com/grafana/grafana/pkg/services/store/object/dummy"
	"github.com/grafana/grafana/pkg/setting"
)

const (
	All                   string = "all"
	GRPCServer            string = "grpc-server"
	GRPCServerHealthCheck string = "grpc-server-health-check"
	GRPCServerReflection  string = "grpc-server-reflection"
	Core                  string = "core"
	ObjectStore           string = "object-store"
)

type Modules struct {
	cfg *setting.Cfg
	log log.Logger

	grpcServer                grpcserver.Provider
	kindRegistry              kind.KindRegistry
	backgroundServiceRegistry *backgroundsvcs.BackgroundServiceRegistry

	ModuleManager  *modules.Manager
	ServiceManager *services.Manager
	serviceMap     map[string]services.Service
	deps           map[string][]string
}

func ProvideService(cfg *setting.Cfg, server grpcserver.Provider, kindRegistry kind.KindRegistry, backgroundServiceRegistry *backgroundsvcs.BackgroundServiceRegistry, roleRegistry accesscontrol.RoleRegistry) *Modules {
	m := &Modules{
		cfg: cfg,
		log: log.New("modules"),

		grpcServer:                server,
		kindRegistry:              kindRegistry,
		backgroundServiceRegistry: backgroundServiceRegistry,
	}
	if err := roleRegistry.RegisterFixedRoles(context.Background()); err != nil {
		panic(err)
	}
	return m
}

func (m *Modules) Init() error {
	mm := modules.NewManager(m.log)
	mm.RegisterModule(GRPCServer, m.initGRPCServer, modules.UserInvisibleModule)
	mm.RegisterModule(GRPCServerHealthCheck, m.initGRPCServerHealthCheck, modules.UserInvisibleModule)
	mm.RegisterModule(GRPCServerReflection, m.initGRPCServerReflection, modules.UserInvisibleModule)
	mm.RegisterModule(ObjectStore, m.initObjectStore)
	mm.RegisterModule(Core, m.initBackgroundServices)
	mm.RegisterModule(All, nil)

	deps := map[string][]string{
		GRPCServer:  {GRPCServerHealthCheck, GRPCServerReflection},
		ObjectStore: {GRPCServer},
		Core:        {},
		All:         {Core, ObjectStore},
	}

	for mod, targets := range deps {
		if err := mm.AddDependency(mod, targets...); err != nil {
			return err
		}
	}

	m.deps = deps
	m.ModuleManager = mm

	return nil
}

func (m *Modules) Run() error {
	serviceMap, err := m.ModuleManager.InitModuleServices(m.cfg.Target...)
	if err != nil {
		return err
	}
	m.serviceMap = serviceMap

	var servs []services.Service
	var keys []string
	for key, s := range serviceMap {
		keys = append(keys, key)
		servs = append(servs, s)
	}

	m.log.Info("starting services", "services", keys)
	sm, err := services.NewManager(servs...)
	if err != nil {
		return err
	}

	m.ServiceManager = sm

	healthy := func() { m.log.Info("msg", "Modules started") }
	stopped := func() { m.log.Info("msg", "Modules stopped") }
	serviceFailed := func(service services.Service) {
		// if any service fails, stop all services
		sm.StopAsync()

		// log which module failed
		for module, s := range serviceMap {
			if s == service {
				if service.FailureCase() == modules.ErrStopProcess {
					m.log.Info("msg", "received stop signal via return error", "module", module, "error", service.FailureCase())
				} else {
					m.log.Error("msg", "module failed", "module", module, "error", service.FailureCase())
				}
				return
			}
		}

		m.log.Error("msg", "module failed", "module", "unknown", "error", service.FailureCase())
	}

	sm.AddListener(services.NewManagerListener(healthy, stopped, serviceFailed))

	// wait until a service fails or we receive a stop signal
	err = sm.StartAsync(context.Background())
	if err == nil {
		return sm.AwaitStopped(context.Background())
	}

	return err
}

func (m *Modules) Stop() error {
	if m.ServiceManager != nil {
		m.ServiceManager.StopAsync()
		return m.ServiceManager.AwaitStopped(context.Background())
	}
	return nil
}

func (m *Modules) initBackgroundServices() (services.Service, error) {
	return m.backgroundServiceRegistry, nil
}

func (m *Modules) initGRPCServer() (services.Service, error) {
	return m.grpcServer, nil
}

func (m *Modules) initGRPCServerHealthCheck() (services.Service, error) {
	return grpcserver.ProvideHealthService(m.cfg, m.grpcServer)
}

func (m *Modules) initGRPCServerReflection() (services.Service, error) {
	return grpcserver.ProvideReflectionService(m.cfg, m.grpcServer)
}

func (m *Modules) initObjectStore() (services.Service, error) {
	return objectdummyserver.ProvideDummyObjectServer(m.cfg, m.grpcServer, m.kindRegistry), nil
}
