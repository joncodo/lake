/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main // must be main for plugin entry point

import (
	"github.com/apache/incubator-devlake/config"
	"github.com/apache/incubator-devlake/logger"
	"github.com/apache/incubator-devlake/plugins/gitlab/impl"
	"github.com/apache/incubator-devlake/plugins/tapd/models"
	"github.com/apache/incubator-devlake/runner"
	"github.com/spf13/cobra"
)

// Export a variable named PluginEntry for Framework to search and load
var PluginEntry impl.Gitlab //nolint

// standalone mode for debugging
func main() {
	gitlabCmd := &cobra.Command{Use: "gitlab"}
	gitlabCmd.Run = func(c *cobra.Command, args []string) {
		cfg := config.GetConfig()
		log := logger.Global.Nested(gitlabCmd.Use)
		db, err := runner.NewGormDb(cfg, log)
		if err != nil {
			panic(err)
		}
		wsList := make([]*models.TapdWorkspace, 0)
		err = db.First(&wsList, "parent_id = ?", 59169984).Error
		if err != nil {
			panic(err)
		}
		projectList := []uint64{63281714,
			34276182,
			46319043,
			50328292,
			63984859,
			55805854,
			38496185,
		}
		for _, v := range projectList {
			runner.DirectRun(gitlabCmd, args, PluginEntry, map[string]interface{}{
				"projectId": v,
			})
		}
	}

	runner.RunCmd(gitlabCmd)
}
