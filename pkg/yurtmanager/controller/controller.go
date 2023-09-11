/*
Copyright 2023 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/controller-manager/app"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/openyurtio/openyurt/cmd/yurt-manager/app/config"
	"github.com/openyurtio/openyurt/cmd/yurt-manager/names"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/csrapprover"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/daemonpodupdater"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/nodepool"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/platformadmin"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/raven/dns"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/raven/gatewayinternalservice"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/raven/gatewaypickup"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/raven/gatewaypublicservice"
	servicetopologyendpoints "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/servicetopology/endpoints"
	servicetopologyendpointslice "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/servicetopology/endpointslice"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtappdaemon"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtappset"
	yurtcoordinatorcert "github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtcoordinator/cert"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtcoordinator/delegatelease"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtcoordinator/podbinding"
	"github.com/openyurtio/openyurt/pkg/yurtmanager/controller/yurtstaticset"
)

type InitFunc func(*config.CompletedConfig, manager.Manager) error

type ControllerInitializersFunc func() (initializers map[string]InitFunc)

var (
	_ ControllerInitializersFunc = NewControllerInitializers

	// ControllersDisabledByDefault is the set of controllers which is disabled by default
	ControllersDisabledByDefault = sets.NewString()
)

// KnownControllers returns all known controllers's name
func KnownControllers() []string {
	ret := sets.StringKeySet(NewControllerInitializers())

	return ret.List()
}

func NewControllerInitializers() map[string]InitFunc {
	controllers := map[string]InitFunc{}
	register := func(name string, fn InitFunc) {
		if _, found := controllers[name]; found {
			panic(fmt.Sprintf("controller name %q was registered twice", name))
		}
		controllers[name] = fn
	}

	register(names.CsrApproverController, csrapprover.Add)
	register(names.DaemonPodUpdaterController, daemonpodupdater.Add)
	register(names.DelegateLeaseController, delegatelease.Add)
	register(names.PodBindingController, podbinding.Add)
	register(names.NodePoolController, nodepool.Add)
	register(names.YurtCoordinatorCertController, yurtcoordinatorcert.Add)
	register(names.ServiceTopologyEndpointsController, servicetopologyendpoints.Add)
	register(names.ServiceTopologyEndpointSliceController, servicetopologyendpointslice.Add)
	register(names.YurtStaticSetController, yurtstaticset.Add)
	register(names.YurtAppSetController, yurtappset.Add)
	register(names.YurtAppDaemonController, yurtappdaemon.Add)
	register(names.PlatformAdminController, platformadmin.Add)
	register(names.GatewayPickupController, gatewaypickup.Add)
	register(names.GatewayDNSController, dns.Add)
	register(names.GatewayInternalServiceController, gatewayinternalservice.Add)
	register(names.GatewayPublicServiceController, gatewaypublicservice.Add)

	return controllers
}

// If you want to add additional RBAC, enter it here !!! @kadisi

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func SetupWithManager(c *config.CompletedConfig, m manager.Manager) error {
	for controllerName, fn := range NewControllerInitializers() {
		if !app.IsControllerEnabled(controllerName, ControllersDisabledByDefault, c.ComponentConfig.Generic.Controllers) {
			klog.Warningf("Controller %v is disabled", controllerName)
			continue
		}

		if err := fn(c, m); err != nil {
			if kindMatchErr, ok := err.(*meta.NoKindMatchError); ok {
				klog.Infof("CRD %v is not installed, its controller will perform noops!", kindMatchErr.GroupKind)
				continue
			}
			return err
		}
	}

	return nil
}