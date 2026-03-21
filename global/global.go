package global

const (
	ZyloFile    = "ZyloFile"
	ZyPth       = "/var/lib/zylo"
	CtrsPth     = "/var/lib/zylo/containers"
	ImgsPth     = "/var/lib/zylo/images"
	NetPth      = "/var/lib/zylo/networks"
	VolPth      = "/var/lib/zylo/volumes"
	ManifestPth = "/var/lib/zylo/manifests"

	CtrsCfgPth  = "/var/run/zylo/containers"
	VolCfgPth   = "/var/run/zylo/volumes"
	NameCfgPth  = "/var/run/zylo/names"
	HostsCfgPth = "/var/run/zylo/hosts"

	MainNetName = "zylo0"

	DnsPort      = ":53"
	DnsExtraPort = ":54"

	SocketPath        = "/var/run/zylo.sock"
	SocketNetworkType = "unix"
	DaemonPIDPath     = "/var/run/zylod.pid"
)
