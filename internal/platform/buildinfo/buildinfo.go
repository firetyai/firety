package buildinfo

type Metadata struct {
	Version string
	Commit  string
	Date    string
}

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func Current() Metadata {
	return Metadata{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}
