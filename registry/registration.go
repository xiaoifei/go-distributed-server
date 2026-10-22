package registry

/**
* ServiceHookURL: 注册中心注册完RequiredServices中的服务后，返回给该地址Entry
 */
type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	RequiredServices []ServiceName
	ServiceHookURL   string
}

type ServiceName string

const (
	LogService        = ServiceName("LogService")
	SeperationService = ServiceName("SeperationService")
)

type patchEntry struct {
	Name ServiceName
	URL  string
}

type patch struct {
	Added   []patchEntry
	Removed []patchEntry
}
