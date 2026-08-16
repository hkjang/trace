package version

var (
	Version   = "0.1.1-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

func Current() Info {
	return Info{Service: "trace", Version: Version, Commit: Commit, BuildTime: BuildTime}
}
