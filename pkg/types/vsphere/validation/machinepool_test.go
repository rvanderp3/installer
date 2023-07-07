package validation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/vsphere"
)

func poolWithHosts(hostCount int, replicas int64, name string) *types.MachinePool {
	hosts := []*vsphere.Host{}
	for idx := 1; idx <= hostCount; idx++ {
		hosts = append(hosts, &vsphere.Host{
			NetworkDevice: &vsphere.NetworkDeviceSpec{
				IPAddrs: []string{
					fmt.Sprintf("192.168.101.%d/24", idx),
				},
				Gateway: "192.168.101.1",
				Nameservers: []string{
					"192.168.101.2",
				},
			},
		})
	}

	return &types.MachinePool{
		Name:     name,
		Replicas: &replicas,
		Platform: types.MachinePoolPlatform{
			VSphere: &vsphere.MachinePool{
				Hosts: hosts,
			},
		},
	}
}

func TestValidateMachinePool(t *testing.T) {
	cases := []struct {
		name           string
		pool           *types.MachinePool
		platform       *vsphere.Platform
		expectedErrMsg string
		expectedZones  *[]string
	}{
		{
			name: "empty",
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{},
				},
			},
			platform:       validPlatform(),
			expectedErrMsg: "",
		}, {
			name:     "negative disk size",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						OSDisk: vsphere.OSDisk{
							DiskSizeGB: -1,
						},
					},
				},
			},
			expectedErrMsg: `^test-path\.diskSizeGB: Invalid value: -1: storage disk size must be positive$`,
		}, {
			name:     "negative CPUs",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						NumCPUs: -1,
					},
				},
			},
			expectedErrMsg: `^test-path\.cpus: Invalid value: -1: number of CPUs must be positive$`,
		}, {
			name:     "negative cores",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						NumCoresPerSocket: -1,
					},
				},
			},
			expectedErrMsg: `^test-path\.coresPerSocket: Invalid value: -1: cores per socket must be positive$`,
		}, {
			name:     "negative memory",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						MemoryMiB: -1,
					},
				},
			},
			expectedErrMsg: `^test-path\.memoryMB: Invalid value: -1: memory size must be positive$`,
		}, {
			name:     "less CPUs than cores per socket",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						NumCPUs:           1,
						NumCoresPerSocket: 8,
					},
				},
			},
			expectedErrMsg: `^test-path\.coresPerSocket: Invalid value: 8: cores per socket must be less than the number of CPUs \(which is by default \d+\)$`,
		},
		{
			name:     "numCPUs not a multiple of cores per socket",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						NumCPUs:           7,
						NumCoresPerSocket: 4,
					},
				},
			},
			expectedErrMsg: `^test-path.cpus: Invalid value: 7: numCPUs specified should be a multiple of cores per socket \(which is by default \d+\)$`,
		},
		{
			name:     "numCPUs not a multiple of default cores per socket",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						NumCPUs: 7,
					},
				},
			},
			expectedErrMsg: `^test-path.cpus: Invalid value: 7: numCPUs specified should be a multiple of cores per socket \(which is by default \d+\)$`,
		},
		{
			name: "multi-zone invalid zone name",
			platform: func() *vsphere.Platform {
				platform := validPlatform()
				platform.FailureDomains[0].Name = "Zone%^@112233"
				return platform
			}(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						Zones: []string{
							"Zone%^@112233",
						},
					},
				},
			},
			expectedErrMsg: `^test-path.zones: Invalid value: \[\]string{"Zone%\^@112233"}: cluster name must begin with a lower-case letter$`,
		},
		{
			name:     "multi-zone valid",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						Zones: []string{
							"test-east-1a",
						},
					},
				},
			},
		},
		{
			name:     "multi-zone no zones defined for control plane pool",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Name: types.MachinePoolControlPlaneRoleName,
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{},
				},
			},
			expectedZones:  &[]string{"test-east-1a", "test-east-2a"},
			expectedErrMsg: "",
		},
		{
			name:     "multi-zone no zones defined for compute pool",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Name: types.MachinePoolComputeRoleName,
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{},
				},
			},
			expectedZones:  &[]string{"test-east-1a", "test-east-2a"},
			expectedErrMsg: "",
		},
		{
			name:     "multi-zone undefined zone",
			platform: validPlatform(),
			pool: &types.MachinePool{
				Platform: types.MachinePoolPlatform{
					VSphere: &vsphere.MachinePool{
						Zones: []string{
							"unknown-zone",
						},
					},
				},
			},
			expectedErrMsg: `^test-path.zones: Invalid value: "unknown-zone": zone not defined in failureDomains$`,
		},
		{
			name: "Static IP - valid",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				return p
			}(),
			pool: poolWithHosts(4, 3, "master"),
		},
		{
			name: "Static IP - no hosts configured",
			platform: func() *vsphere.Platform {
				p := validPlatform()
				return p
			}(),
			pool: poolWithHosts(0, 3, "master"),
		},
		{
			name: "Static IP - invalid FailureDomain",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].FailureDomain = "north-pole"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.failureDomain: Invalid value: "north-pole": failure domain not found$`,
		},
		{
			name: "Static IP - missing NetworkDevice",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice = nil
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.networkDevice: Required value: must specify networkDevice configuration$`,
		},
		{
			name: "Static IP - missing IP",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.IPAddrs = nil
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.ipAddrs: Required value: must specify a IP$`,
		},
		{
			name: "Static IP - invalid IP",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309/24"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309/24": invalid CIDR address: 86.7.5.309/24$`,
		},
		{
			name: "Static IP - invalid IP blank",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.IPAddrs[0] = ""
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.ipAddrs: Required value: must specify a IP address with CIDR$`,
		},
		{
			name: "Static IP - invalid IP CIDR",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309/55"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309/55": invalid CIDR address: 86.7.5.309/55$`,
		},
		{
			name: "Static IP - invalid IP missing CIDR",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.IPAddrs[0] = "86.7.5.309"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.ipAddrs: Invalid value: "86.7.5.309": invalid CIDR address: 86.7.5.309$`,
		},
		{
			name: "Static IP - valid Gateway IPv4",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.Gateway = "192.168.100.125"
				return pool
			}(),
		},
		{
			name: "Static IP - invalid Gateway IPv4",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.Gateway = "86.7.5.309"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.gateway: Invalid value: "86.7.5.309": "86.7.5.309" is not a valid IP$`,
		},
		{
			name: "Static IP - valid Gateway IPv6",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.Gateway = "2001:db8:3333:4444:5555:6666:7777:8888"
				return pool
			}(),
		},
		{
			name: "Static IP - invalid Gateway IPv6",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.Gateway = "8888:666:7777:5555:3333:0000:9999:JENNY"
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.gateway: Invalid value: "8888:666:7777:5555:3333:0000:9999:JENNY": "8888:666:7777:5555:3333:0000:9999:JENNY" is not a valid IP$`,
		},
		{
			name: "Static IP - More than 3 nameservers",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "master")
				pool.Platform.VSphere.Hosts[1].NetworkDevice.Nameservers = []string{"86.75.30.9", "86.75.30.8", "86.75.30.7", "86.75.30.6"}
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts.nameservers: Too many: 4: must have at most 3 items$`,
		},
		{
			name: "Static IP - Not enough control-planes",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(3, 3, "master")
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts: Invalid value: "master": not enough hosts found \(3\) to support all the configured master machine pool replicas \(4\)$`,
		},
		{
			name: "Static IP - Too many control-planes",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(5, 3, "master")
				return pool
			}(),
		},
		{
			name: "Static IP - Not enough workers",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(2, 3, "worker")
				return pool
			}(),
			expectedErrMsg: `^test-path.hosts: Invalid value: "worker": not enough hosts found \(2\) to support all the configured worker machine pool replicas \(3\)$`,
		},
		{
			name: "Static IP - Too many workers",
			pool: func() *types.MachinePool {
				pool := poolWithHosts(4, 3, "worker")
				return pool
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.platform == nil {
				tc.platform = validPlatform()
			}
			err := ValidateMachinePool(tc.platform, tc.pool, field.NewPath("test-path")).ToAggregate()
			if tc.expectedErrMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexp(t, tc.expectedErrMsg, err)
			}
			if tc.expectedZones != nil {
				zones := tc.pool.Platform.VSphere.Zones
				for _, expectedZone := range *tc.expectedZones {
					found := false
					for _, zone := range zones {
						if zone == expectedZone {
							found = true
							break
						}
					}
					if found == false {
						t.Errorf("expected zone not found %s", expectedZone)
					}
				}
				for _, zone := range zones {
					found := false
					for _, expectedZone := range *tc.expectedZones {
						if zone == expectedZone {
							found = true
							break
						}
					}
					if found == false {
						t.Errorf("unexpected zone %s", zone)
					}
				}
			}
		})
	}
}
