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

//+build !windows

package vfio_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/networkservicemesh/api/pkg/api/networkservice/mechanisms/cls"
	vfiomech "github.com/networkservicemesh/api/pkg/api/networkservice/mechanisms/vfio"
	"github.com/networkservicemesh/sdk/pkg/networkservice/common/mechanisms"
	"github.com/networkservicemesh/sdk/pkg/networkservice/core/chain"
	"github.com/stretchr/testify/assert"

	"github.com/networkservicemesh/sdk-sriov/pkg/networkservice/common/mechanisms/vfio"
	"github.com/networkservicemesh/sdk-sriov/pkg/sriov/sriovtest"
)

const (
	deviceAllowFile    = "devices.allow"
	deviceDenyFile     = "devices.deny"
	deviceStringFormat = "c %d:%d rwm"
)

func testVFIOServer(ctx context.Context, t *testing.T, allowedDevices *allowedDevices) (server networkservice.NetworkServiceServer, tmpDir string) {
	tmpDir = path.Join(os.TempDir(), t.Name())
	err := os.MkdirAll(path.Join(tmpDir, cgroupDir), 0750)
	assert.Nil(t, err)

	server = chain.NewNetworkServiceServer(
		mechanisms.NewServer(map[string]networkservice.NetworkServiceServer{
			vfiomech.MECHANISM: vfio.NewServer(tmpDir, tmpDir),
		}),
	)

	err = sriovtest.InputFileAPI(ctx, path.Join(tmpDir, cgroupDir, deviceAllowFile), func(s string) {
		var major, minor int
		_, _ = fmt.Sscanf(s, deviceStringFormat, &major, &minor)
		allowedDevices.Lock()
		allowedDevices.devices[fmt.Sprintf("%d:%d", major, minor)] = true
		allowedDevices.Unlock()
	})
	assert.Nil(t, err)
	err = sriovtest.InputFileAPI(ctx, path.Join(tmpDir, cgroupDir, deviceDenyFile), func(s string) {
		var major, minor int
		_, _ = fmt.Sscanf(s, deviceStringFormat, &major, &minor)
		allowedDevices.Lock()
		delete(allowedDevices.devices, fmt.Sprintf("%d:%d", major, minor))
		allowedDevices.Unlock()
	})
	assert.Nil(t, err)

	return server, tmpDir
}

func TestVFIOServer_Request(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	allowedDevices := &allowedDevices{
		devices: map[string]bool{},
	}
	server, tmpDir := testVFIOServer(ctx, t, allowedDevices)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	err := unix.Mknod(path.Join(tmpDir, vfioDevice), unix.S_IFCHR|0666, int(unix.Mkdev(1, 2)))
	assert.Nil(t, err)
	err = unix.Mknod(path.Join(tmpDir, iommuGroupString), unix.S_IFCHR|0666, int(unix.Mkdev(3, 4)))
	assert.Nil(t, err)

	conn, err := server.Request(ctx, &networkservice.NetworkServiceRequest{
		Connection: &networkservice.Connection{},
		MechanismPreferences: []*networkservice.Mechanism{
			{
				Cls:  cls.LOCAL,
				Type: vfiomech.MECHANISM,
				Parameters: map[string]string{
					vfiomech.CgroupDirKey:  cgroupDir,
					vfiomech.IommuGroupKey: iommuGroupString,
				},
			},
		},
	})
	assert.Nil(t, err)

	mech := vfiomech.ToMechanism(conn.GetMechanism())
	assert.NotNil(t, mech)
	assert.Equal(t, uint32(1), mech.GetVfioMajor())
	assert.Equal(t, uint32(2), mech.GetVfioMinor())
	assert.Equal(t, uint32(3), mech.GetDeviceMajor())
	assert.Equal(t, uint32(4), mech.GetDeviceMinor())

	assert.Eventually(t, func() bool {
		allowedDevices.Lock()
		defer allowedDevices.Unlock()
		return reflect.DeepEqual(map[string]bool{
			"1:2": true,
			"3:4": true,
		}, allowedDevices.devices)
	}, time.Second, 10*time.Millisecond)

	assert.Nil(t, ctx.Err())
}

func TestVFIOServer_Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	allowedDevices := &allowedDevices{
		devices: map[string]bool{
			"1:2": true,
			"3:4": true,
		},
	}
	server, tmpDir := testVFIOServer(ctx, t, allowedDevices)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	conn := &networkservice.Connection{
		Mechanism: &networkservice.Mechanism{
			Cls:  cls.LOCAL,
			Type: vfiomech.MECHANISM,
			Parameters: map[string]string{
				vfiomech.CgroupDirKey:   cgroupDir,
				vfiomech.IommuGroupKey:  iommuGroupString,
				vfiomech.VfioMajorKey:   "1",
				vfiomech.VfioMinorKey:   "2",
				vfiomech.DeviceMajorKey: "3",
				vfiomech.DeviceMinorKey: "4",
			},
		},
	}

	_, err := server.Close(ctx, conn)
	assert.Nil(t, err)

	assert.Eventually(t, func() bool {
		allowedDevices.Lock()
		defer allowedDevices.Unlock()
		return reflect.DeepEqual(map[string]bool{}, allowedDevices.devices)
	}, time.Second, 10*time.Millisecond)

	assert.Nil(t, ctx.Err())
}

type allowedDevices struct {
	devices map[string]bool

	sync.Mutex
}
