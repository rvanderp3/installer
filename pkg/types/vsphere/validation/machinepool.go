package validation

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/vsphere"
	"github.com/openshift/installer/pkg/validate"
)

const (
	defaultCoresPerSocket = int32(4)
	defaultNumCPUs        = int32(4)
)

// ValidateMachinePool checks that the specified machine pool is valid.
func ValidateMachinePool(platform *vsphere.Platform, machinePool *types.MachinePool, fldPath *field.Path) field.ErrorList {
	vspherePool := machinePool.Platform.VSphere
	allErrs := field.ErrorList{}
	if vspherePool.DiskSizeGB < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("diskSizeGB"), vspherePool.DiskSizeGB, "storage disk size must be positive"))
	}
	if vspherePool.MemoryMiB < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("memoryMB"), vspherePool.MemoryMiB, "memory size must be positive"))
	}
	numCPUs := vspherePool.NumCPUs
	if numCPUs < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cpus"), numCPUs, "number of CPUs must be positive"))
	}
	numCoresPerSocket := vspherePool.NumCoresPerSocket
	if numCoresPerSocket < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("coresPerSocket"), numCoresPerSocket, "cores per socket must be positive"))
	}

	// Either the number set by the user or a default value
	if numCPUs <= 0 {
		numCPUs = defaultNumCPUs
	}
	if numCoresPerSocket <= 0 {
		numCoresPerSocket = defaultCoresPerSocket
	}

	if numCoresPerSocket > numCPUs {
		errorMsg := fmt.Sprintf("cores per socket must be less than the number of CPUs (which is by default %d)", defaultNumCPUs)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("coresPerSocket"), numCoresPerSocket, errorMsg))
	} else if numCPUs%numCoresPerSocket != 0 {
		errMsg := fmt.Sprintf("numCPUs specified should be a multiple of cores per socket (which is by default %d)", defaultCoresPerSocket)
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cpus"), numCPUs, errMsg))
	}

	if len(vspherePool.Zones) > 0 {
		if len(platform.FailureDomains) == 0 {
			return append(allErrs, field.Required(fldPath.Child("zones"), "failureDomains must be defined if zones are defined"))
		}
		for _, zone := range vspherePool.Zones {
			err := validate.ClusterName1035(zone)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("zones"), vspherePool.Zones, err.Error()))
			}
			zoneDefined := false
			for _, failureDomain := range platform.FailureDomains {
				if failureDomain.Name == zone {
					zoneDefined = true
				}
			}
			if !zoneDefined {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("zones"), zone, "zone not defined in failureDomains"))
			}
		}
	} else if len(platform.FailureDomains) > 0 {
		for _, failureDomain := range platform.FailureDomains {
			vspherePool.Zones = append(vspherePool.Zones, failureDomain.Name)
		}
	}
	if len(vspherePool.Hosts) > 0 {
		validateHosts(platform, machinePool)
		//allErrs = append(allErrs, ) )
	}
	return allErrs
}

// validateHosts.
func validateHosts(platform *vsphere.Platform, pool *types.MachinePool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	hosts := pool.Platform.VSphere.Hosts
	// Validate hosts counts match desired replicas
	allErrs = append(allErrs, validateHostsCount(pool, fldPath)...)

	// Iterate through hosts
	for _, host := range hosts {
		// Check failure domain (must exist in failure domains)
		if host.FailureDomain != "" {
			allErrs = append(allErrs, validateHostFailureDomain(host, platform.FailureDomains, fldPath)...)
		}

		// Check networking
		if host.NetworkDevice == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("networkDevice"), "must specify networkDevice configuration"))
		} else {
			allErrs = append(allErrs, validateHostNetworking(host.NetworkDevice, fldPath)...)
		}
	}

	return allErrs
}

// validateHostsCount ensure that the number of hosts is enough to cover the requirements of the
// machine pool
func validateHostsCount(pool *types.MachinePool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	requiredHosts := *pool.Replicas
	if pool.Name == types.MachinePoolControlPlaneRoleName {
		requiredHosts++
	}

	numberOfHosts := int64(len(pool.Platform.VSphere.Hosts))
	if numberOfHosts < requiredHosts {
		errMsg := fmt.Sprintf("not enough hosts found (%v) to support all the configured machine pool replicas (%v)", numberOfHosts, requiredHosts)
		allErrs = append(allErrs, field.Invalid(fldPath, "machine-pool", errMsg))
	} else if numberOfHosts > requiredHosts {
		errMsg := fmt.Sprintf("too many hosts found (%v) for the configured machine pool (%v)", numberOfHosts, requiredHosts)
		allErrs = append(allErrs, field.Invalid(fldPath, "machine-pool", errMsg))
	}
	return allErrs
}

// validateHostFailureDomain returns error if the FailureDomain is not found.
func validateHostFailureDomain(host *vsphere.Host, fds []vsphere.FailureDomain, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	fdFound := false
	for _, domain := range fds {
		if domain.Name == host.FailureDomain {
			fdFound = true
			break
		}
	}
	if !fdFound {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("failureDomain"), host.FailureDomain, "failure domain not found"))
	}
	return allErrs
}

// validateHostNetworking checks all fields related to networking for a host.  If any errors are found, they will
// be returned (invalid IP, IP required).
func validateHostNetworking(network *vsphere.NetworkDeviceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check ip addresses
	if len(network.IPAddrs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("ipAddrs"), "must specify a IP"))
	}
	for _, ip := range network.IPAddrs {
		allErrs = append(allErrs, validateIPWithCidr(ip, true, fldPath.Child("ipAddrs"))...)
	}

	// Check nameservers
	if len(network.Nameservers) > 3 {
		allErrs = append(allErrs, field.TooMany(fldPath.Child("nameservers"), len(network.Nameservers), 3))
	}
	for _, nameserver := range network.Nameservers {
		allErrs = append(allErrs, validateIP(nameserver, false, fldPath.Child("nameservers"))...)
	}

	// Check gateway
	allErrs = append(allErrs, validateIP(network.Gateway, false, fldPath.Child("gateway"))...)

	return allErrs
}

// validateIPWithCidr checks IP/CIDR value to see if it is valid.  If IP is required, an error will be returned if
// the IP is not specified.
func validateIPWithCidr(ip string, req bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ip == "" && req {
		allErrs = append(allErrs, field.Required(fldPath, "must specify a IP address with CIDR"))
	} else if ip != "" {
		if _, _, valErr := net.ParseCIDR(ip); valErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, ip, valErr.Error()))
		}
	}
	return allErrs
}

// validateIP checks IP value to see if it is valid.  If IP is required, an error will be returned if
// the IP is not specified.
func validateIP(ip string, req bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ip == "" && req {
		allErrs = append(allErrs, field.Required(fldPath, "must specify a IP"))
	} else if ip != "" {
		if valErr := validate.IP(ip); valErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, ip, valErr.Error()))
		}
	}
	return allErrs
}
