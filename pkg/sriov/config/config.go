// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config provides SR-IOV config
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/networkservicemesh/sdk/pkg/tools/log"

	"github.com/networkservicemesh/sdk-sriov/pkg/tools/yamlhelper"
)

// Config contains list of available physical functions
type Config struct {
	PhysicalFunctions map[string]*PhysicalFunction `yaml:"physicalFunctions"`
}

func (c *Config) String() string {
	sb := &strings.Builder{}
	_, _ = sb.WriteString("&{PhysicalFunctions:map[")
	for k, physicalFunction := range c.PhysicalFunctions {
		_, _ = sb.WriteString(fmt.Sprintf("%s:%+v ", k, physicalFunction))
	}
	_, _ = sb.WriteString("]}")
	return sb.String()
}

// PhysicalFunction contains physical function capabilities, available services domains and virtual functions IOMMU groups
type PhysicalFunction struct {
	Capabilities     []string        `yaml:"capabilities"`
	ServiceDomains   []string        `yaml:"serviceDomains"`
	VirtualFunctions map[string]uint `yaml:"virtualFunctions"`
}

// ReadConfig reads configuration from file
func ReadConfig(ctx context.Context, configFile string) (*Config, error) {
	logEntry := log.Entry(ctx).WithField("Config", "ReadConfig")

	config := &Config{}
	if err := yamlhelper.UnmarshalFile(configFile, config); err != nil {
		return nil, err
	}

	logEntry.Infof("unmarshalled Config: %+v", config)

	return config, nil
}
