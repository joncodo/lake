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

package models

import (
	"github.com/apache/incubator-devlake/models/common"
	"github.com/apache/incubator-devlake/plugins/helper"
)

type TapdWorklog struct {
	ConnectionId uint64         `gorm:"primaryKey;type:BIGINT  NOT NULL"`
	ID           uint64         `gorm:"primaryKey;type:BIGINT  NOT NULL" json:"id,string"`
	WorkspaceID  uint64         `json:"workspace_id,string"`
	EntityType   string         `gorm:"type:varchar(255)" json:"entity_type"`
	EntityID     uint64         `json:"entity_id,string"`
	Timespent    float32        `json:"timespent,string"`
	Spentdate    helper.CSTTime `json:"spentdate"`
	Owner        string         `gorm:"type:varchar(255)" json:"owner"`
	Created      helper.CSTTime `json:"created"`
	Memo         string         `json:"memo" gorm:"type:text"`
	common.NoPKModel
}

func (TapdWorklog) TableName() string {
	return "_tool_tapd_worklogs"
}
