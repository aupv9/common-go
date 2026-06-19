package health

// Build metadata — set lúc build qua -ldflags, ví dụ:
//
//	go build -ldflags "\
//	  -X github.com/aupv9/common-go/health.Version=$(git describe --tags) \
//	  -X github.com/aupv9/common-go/health.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/aupv9/common-go/health.BuildTime=$(date -u +%FT%TZ)"
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// BuildInfo là payload của endpoint /info.
type BuildInfo struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	Env       string `json:"env"`
}
