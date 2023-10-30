/*
Copyright helen-frank

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

package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/helen-frank/hcnmp/pkg/apis/config"
	"github.com/helen-frank/hcnmp/pkg/server/handlers/clusters"
	"github.com/helen-frank/hcnmp/pkg/server/handlers/server"
	"github.com/helen-frank/hcnmp/pkg/server/middleware/auth"
	"github.com/helen-frank/hcnmp/pkg/server/middleware/monitor/prom"
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
)

type Server struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	engine *gin.Engine
	client clientset.Interface
}

func Run(cfg *config.Config, client clientset.Interface) error {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
		client: client,
		engine: gin.Default(),
	}

	s.InstallHandlers()

	return s.engine.Run(":" + strconv.Itoa(s.cfg.Port))
}

func (s *Server) InstallHandlers() {
	if !s.cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	} else {
		pprof.Register(s.engine)
	}

	// install healthz
	s.engine.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "happy everyday")
	})

	s.engine.Use(prom.PromMiddleware(nil), gin.Recovery())
	s.engine.GET("/metrics", prom.PromHandler(promhttp.Handler()))

	authorized := s.engine.Group("/", auth.MultiAuth(gin.Accounts{
		s.cfg.BasicAuthUser: s.cfg.BasicAuthPassword,
	}))

	apiGroup := authorized.Group("/apis")
	{
		clusters.InstallHandlers(apiGroup.Group("/cluster"), s.cfg.NameSpace, s.cfg.ClusterInfos, s.cfg.LocalClusterInfos, s.client)
		server.InstallHandlers(apiGroup.Group("/server"), s.client)
	}

}
