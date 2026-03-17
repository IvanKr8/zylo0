package global

var (
	ZyloFile    = "ZyloFile"
	ZyPth       = "/var/lib/zylo"
	CtrsPth     = "/var/lib/zylo/containers"
	ImgsPth     = "/var/lib/zylo/images"
	NetPth      = "/var/lib/zylo/networks"
	VolPth      = "/var/lib/zylo/volumes"
	ManifestPth = "/var/lib/zylo/manifests"

	CtrsCfgPth = "/var/run/zylo/containers"
	VolCfgPth  = "/var/run/zylo/volumes"

	BaseImage   = "ubuntu:rootfs"
	MainNetName = "zylo0"

	TTY string

	ApiBaseUrl = "http://localhost:8080"
	ApiVer     = "/api/v1"

	ApiPing         = ApiVer + "/ping"
	ApiVerifyImage  = ApiVer + "/verify-image"
	ApiDownloadInfo = ApiVer + "/download-image"
	ApiPullImage    = ApiVer + "/pull"
)
