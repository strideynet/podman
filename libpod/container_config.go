package libpod

import (
	"net"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/pkg/namespaces"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerConfig contains all information that was used to create the
// container. It may not be changed once created.
// It is stored, read-only, on disk in Libpod's State.
// Any changes will not be written back to the database, and will cause
// inconsistencies with other Libpod instances.
type ContainerConfig struct {
	// Spec is OCI runtime spec used to create the container. This is passed
	// in when the container is created, but it is not the final spec used
	// to run the container - it will be modified by Libpod to add things we
	// manage (e.g. bind mounts for /etc/resolv.conf, named volumes, a
	// network namespace prepared by CNI or slirp4netns) in the
	// generateSpec() function.
	Spec *spec.Spec `json:"spec"`

	// ID is a hex-encoded 256-bit pseudorandom integer used as a unique
	// identifier for the container. IDs are globally unique in Libpod -
	// once an ID is in use, no other container or pod will be created with
	// the same one until the holder of the ID has been removed.
	// ID is generated by Libpod, and cannot be chosen or influenced by the
	// user (except when restoring a checkpointed container).
	// ID is guaranteed to be 64 characters long.
	ID string `json:"id"`

	// Name is a human-readable name for the container. All containers must
	// have a non-empty name. Name may be provided when the container is
	// created; if no name is chosen, a name will be auto-generated.
	Name string `json:"name"`

	// Pod is the full ID of the pod the container belongs to. If the
	// container does not belong to a pod, this will be empty.
	// If this is not empty, a pod with this ID is guaranteed to exist in
	// the state for the duration of this container's existence.
	Pod string `json:"pod,omitempty"`

	// Namespace is the libpod Namespace the container is in.
	// Namespaces are used to divide containers in the state.
	Namespace string `json:"namespace,omitempty"`

	// LockID is the ID of this container's lock. Each container, pod, and
	// volume is assigned a unique Lock (from one of several backends) by
	// the libpod Runtime. This lock will belong only to this container for
	// the duration of the container's lifetime.
	LockID uint32 `json:"lockID"`

	// CreateCommand is the full command plus arguments that were used to
	// create the container. It is shown in the output of Inspect, and may
	// be used to recreate an identical container for automatic updates or
	// portable systemd unit files.
	CreateCommand []string `json:"CreateCommand,omitempty"`

	// RawImageName is the raw and unprocessed name of the image when creating
	// the container (as specified by the user).  May or may not be set.  One
	// use case to store this data are auto-updates where we need the _exact_
	// name and not some normalized instance of it.
	RawImageName string `json:"RawImageName,omitempty"`

	// IDMappings are UID/GID mappings used by the container's user
	// namespace. They are used by the OCI runtime when creating the
	// container, and by c/storage to ensure that the container's files have
	// the appropriate owner.
	IDMappings storage.IDMappingOptions `json:"idMappingsOptions,omitempty"`

	// Dependencies are the IDs of dependency containers.
	// These containers must be started before this container is started.
	Dependencies []string

	// embedded sub-configs
	ContainerRootFSConfig
	ContainerSecurityConfig
	ContainerNameSpaceConfig
	ContainerNetworkConfig
	ContainerImageConfig
	ContainerMiscConfig
}

// ContainerRootFSConfig is an embedded sub-config providing config info
// about the container's root fs.
type ContainerRootFSConfig struct {
	// RootfsImageID is the ID of the image used to create the container.
	// If the container was created from a Rootfs, this will be empty.
	// If non-empty, Podman will create a root filesystem for the container
	// based on an image with this ID.
	// This conflicts with Rootfs.
	RootfsImageID string `json:"rootfsImageID,omitempty"`
	// RootfsImageName is the (normalized) name of the image used to create
	// the container. If the container was created from a Rootfs, this will
	// be empty.
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	// Rootfs is a directory to use as the container's root filesystem.
	// If RootfsImageID is set, this will be empty.
	// If this is set, Podman will not create a root filesystem for the
	// container based on an image, and will instead use the given directory
	// as the container's root.
	// Conflicts with RootfsImageID.
	Rootfs string `json:"rootfs,omitempty"`
	// ShmDir is the path to be mounted on /dev/shm in container.
	// If not set manually at creation time, Libpod will create a tmpfs
	// with the size specified in ShmSize and populate this with the path of
	// said tmpfs.
	ShmDir string `json:"ShmDir,omitempty"`
	// ShmSize is the size of the container's SHM. Only used if ShmDir was
	// not set manually at time of creation.
	ShmSize int64 `json:"shmSize"`
	// Static directory for container content that will persist across
	// reboot.
	// StaticDir is a persistent directory for Libpod files that will
	// survive system reboot. It is not part of the container's rootfs and
	// is not mounted into the container. It will be removed when the
	// container is removed.
	// Usually used to store container log files, files that will be bind
	// mounted into the container (e.g. the resolv.conf we made for the
	// container), and other per-container content.
	StaticDir string `json:"staticDir"`
	// Mounts contains all additional mounts into the container rootfs.
	// It is presently only used for the container's SHM directory.
	// These must be unmounted before the container's rootfs is unmounted.
	Mounts []string `json:"mounts,omitempty"`
	// NamedVolumes lists the Libpod named volumes to mount into the
	// container. Each named volume is guaranteed to exist so long as this
	// container exists.
	NamedVolumes []*ContainerNamedVolume `json:"namedVolumes,omitempty"`
	// OverlayVolumes lists the overlay volumes to mount into the container.
	OverlayVolumes []*ContainerOverlayVolume `json:"overlayVolumes,omitempty"`
	// CreateWorkingDir indicates that Libpod should create the container's
	// working directory if it does not exist. Some OCI runtimes do this by
	// default, but others do not.
	CreateWorkingDir bool `json:"createWorkingDir,omitempty"`
}

// ContainerSecurityConfig is an embedded sub-config providing security configuration
// to the container.
type ContainerSecurityConfig struct {
	// Pirivileged is whether the container is privileged. Privileged
	// containers have lessened security and increased access to the system.
	// Note that this does NOT directly correspond to Podman's --privileged
	// flag - most of the work of that flag is done in creating the OCI spec
	// given to Libpod. This only enables a small subset of the overall
	// operation, mostly around mounting the container image with reduced
	// security.
	Privileged bool `json:"privileged"`
	// ProcessLabel is the SELinux process label for the container.
	ProcessLabel string `json:"ProcessLabel,omitempty"`
	// MountLabel is the SELinux mount label for the container's root
	// filesystem. Only used if the container was created from an image.
	// If not explicitly set, an unused random MLS label will be assigned by
	// containers/storage (but only if SELinux is enabled).
	MountLabel string `json:"MountLabel,omitempty"`
	// LabelOpts are options passed in by the user to setup SELinux labels.
	// These are used by the containers/storage library.
	LabelOpts []string `json:"labelopts,omitempty"`
	// User and group to use in the container. Can be specified as only user
	// (in which case we will attempt to look up the user in the container
	// to determine the appropriate group) or user and group separated by a
	// colon.
	// Can be specified by name or UID/GID.
	// If unset, this will default to UID and GID 0 (root).
	User string `json:"user,omitempty"`
	// Groups are additional groups to add the container's user to. These
	// are resolved within the container using the container's /etc/passwd.
	Groups []string `json:"groups,omitempty"`
	// AddCurrentUserPasswdEntry indicates that Libpod should ensure that
	// the container's /etc/passwd contains an entry for the user running
	// Libpod - mostly used in rootless containers where the user running
	// Libpod wants to retain their UID inside the container.
	AddCurrentUserPasswdEntry bool `json:"addCurrentUserPasswdEntry,omitempty"`
}

// ContainerNameSpaceConfig is an embedded sub-config providing
// namespace configuration to the container.
type ContainerNameSpaceConfig struct {
	// IDs of container to share namespaces with
	// NetNsCtr conflicts with the CreateNetNS bool
	// These containers are considered dependencies of the given container
	// They must be started before the given container is started
	IPCNsCtr    string `json:"ipcNsCtr,omitempty"`
	MountNsCtr  string `json:"mountNsCtr,omitempty"`
	NetNsCtr    string `json:"netNsCtr,omitempty"`
	PIDNsCtr    string `json:"pidNsCtr,omitempty"`
	UserNsCtr   string `json:"userNsCtr,omitempty"`
	UTSNsCtr    string `json:"utsNsCtr,omitempty"`
	CgroupNsCtr string `json:"cgroupNsCtr,omitempty"`
}

// ContainerNetworkConfig is an embedded sub-config providing network configuration
// to the container.
type ContainerNetworkConfig struct {
	// CreateNetNS indicates that libpod should create and configure a new
	// network namespace for the container.
	// This cannot be set if NetNsCtr is also set.
	CreateNetNS bool `json:"createNetNS"`
	// StaticIP is a static IP to request for the container.
	// This cannot be set unless CreateNetNS is set.
	// If not set, the container will be dynamically assigned an IP by CNI.
	StaticIP net.IP `json:"staticIP"`
	// StaticMAC is a static MAC to request for the container.
	// This cannot be set unless CreateNetNS is set.
	// If not set, the container will be dynamically assigned a MAC by CNI.
	StaticMAC net.HardwareAddr `json:"staticMAC"`
	// PortMappings are the ports forwarded to the container's network
	// namespace
	// These are not used unless CreateNetNS is true
	PortMappings []ocicni.PortMapping `json:"portMappings,omitempty"`
	// UseImageResolvConf indicates that resolv.conf should not be
	// bind-mounted inside the container.
	// Conflicts with DNSServer, DNSSearch, DNSOption.
	UseImageResolvConf bool
	// DNS servers to use in container resolv.conf
	// Will override servers in host resolv if set
	DNSServer []net.IP `json:"dnsServer,omitempty"`
	// DNS Search domains to use in container resolv.conf
	// Will override search domains in host resolv if set
	DNSSearch []string `json:"dnsSearch,omitempty"`
	// DNS options to be set in container resolv.conf
	// With override options in host resolv if set
	DNSOption []string `json:"dnsOption,omitempty"`
	// UseImageHosts indicates that /etc/hosts should not be
	// bind-mounted inside the container.
	// Conflicts with HostAdd.
	UseImageHosts bool
	// Hosts to add in container
	// Will be appended to host's host file
	HostAdd []string `json:"hostsAdd,omitempty"`
	// Network names (CNI) to add container to. Empty to use default network.
	Networks []string `json:"networks,omitempty"`
	// Network mode specified for the default network.
	NetMode namespaces.NetworkMode `json:"networkMode,omitempty"`
	// NetworkOptions are additional options for each network
	NetworkOptions map[string][]string `json:"network_options,omitempty"`
}

// ContainerImageConfig is an embedded sub-config providing image configuration
// to the container.
type ContainerImageConfig struct {
	// UserVolumes contains user-added volume mounts in the container.
	// These will not be added to the container's spec, as it is assumed
	// they are already present in the spec given to Libpod. Instead, it is
	// used when committing containers to generate the VOLUMES field of the
	// image that is created, and for triggering some OCI hooks which do not
	// fire unless user-added volume mounts are present.
	UserVolumes []string `json:"userVolumes,omitempty"`
	// Entrypoint is the container's entrypoint.
	// It is not used in spec generation, but will be used when the
	// container is committed to populate the entrypoint of the new image.
	Entrypoint []string `json:"entrypoint,omitempty"`
	// Command is the container's command.
	// It is not used in spec generation, but will be used when the
	// container is committed to populate the command of the new image.
	Command []string `json:"command,omitempty"`
}

// ContainerMiscConfig is an embedded sub-config providing misc configuration
// to the container.
type ContainerMiscConfig struct {
	// Whether to keep container STDIN open
	Stdin bool `json:"stdin,omitempty"`
	// Labels is a set of key-value pairs providing additional information
	// about a container
	Labels map[string]string `json:"labels,omitempty"`
	// StopSignal is the signal that will be used to stop the container
	StopSignal uint `json:"stopSignal,omitempty"`
	// StopTimeout is the signal that will be used to stop the container
	StopTimeout uint `json:"stopTimeout,omitempty"`
	// Time container was created
	CreatedTime time.Time `json:"createdTime"`
	// NoCgroups indicates that the container will not create CGroups. It is
	// incompatible with CgroupParent.  Deprecated in favor of CgroupsMode.
	NoCgroups bool `json:"noCgroups,omitempty"`
	// CgroupsMode indicates how the container will create cgroups
	// (disabled, no-conmon, enabled).  It supersedes NoCgroups.
	CgroupsMode string `json:"cgroupsMode,omitempty"`
	// Cgroup parent of the container
	CgroupParent string `json:"cgroupParent"`
	// LogPath log location
	LogPath string `json:"logPath"`
	// LogTag is the tag used for logging
	LogTag string `json:"logTag"`
	// LogSize is the tag used for logging
	LogSize int64 `json:"logSize"`
	// LogDriver driver for logs
	LogDriver string `json:"logDriver"`
	// File containing the conmon PID
	ConmonPidFile string `json:"conmonPidFile,omitempty"`
	// RestartPolicy indicates what action the container will take upon
	// exiting naturally.
	// Allowed options are "no" (take no action), "on-failure" (restart on
	// non-zero exit code, up an a maximum of RestartRetries times),
	// and "always" (always restart the container on any exit code).
	// The empty string is treated as the default ("no")
	RestartPolicy string `json:"restart_policy,omitempty"`
	// RestartRetries indicates the number of attempts that will be made to
	// restart the container. Used only if RestartPolicy is set to
	// "on-failure".
	RestartRetries uint `json:"restart_retries,omitempty"`
	// TODO log options for log drivers
	// PostConfigureNetNS needed when a user namespace is created by an OCI runtime
	// if the network namespace is created before the user namespace it will be
	// owned by the wrong user namespace.
	PostConfigureNetNS bool `json:"postConfigureNetNS"`
	// OCIRuntime used to create the container
	OCIRuntime string `json:"runtime,omitempty"`
	// ExitCommand is the container's exit command.
	// This Command will be executed when the container exits by Conmon.
	// It is usually used to invoke post-run cleanup - for example, in
	// Podman, it invokes `podman container cleanup`, which in turn calls
	// Libpod's Cleanup() API to unmount the container and clean up its
	// network.
	ExitCommand []string `json:"exitCommand,omitempty"`
	// IsInfra is a bool indicating whether this container is an infra container used for
	// sharing kernel namespaces in a pod
	IsInfra bool `json:"pause"`
	// SdNotifyMode tells libpod what to do with a NOTIFY_SOCKET if passed
	SdNotifyMode string `json:"sdnotifyMode,omitempty"`
	// Systemd tells libpod to setup the container in systemd mode
	Systemd bool `json:"systemd"`
	// HealthCheckConfig has the health check command and related timings
	HealthCheckConfig *manifest.Schema2HealthConfig `json:"healthcheck"`
	// PreserveFDs is a number of additional file descriptors (in addition
	// to 0, 1, 2) that will be passed to the executed process. The total FDs
	// passed will be 3 + PreserveFDs.
	PreserveFDs uint `json:"preserveFds,omitempty"`
	// Timezone is the timezone inside the container.
	// Local means it has the same timezone as the host machine
	Timezone string `json:"timezone,omitempty"`
	// Umask is the umask inside the container.
	Umask string `json:"umask,omitempty"`
}
