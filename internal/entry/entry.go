// Package entry is the data model of a registry entry and the rules that make one valid.
package entry

type Kind string

const (
	Tool   Kind = "tool"
	Folder Kind = "folder"
)

type Holds string

const (
	Cache    Holds = "cache"
	Versions Holds = "versions"
)

type MeasureBy string

const (
	DiskUsage MeasureBy = "disk-usage"
	Command   MeasureBy = "command"
)

const (
	PathPlaceholder    = "<path>"
	VersionPlaceholder = "<version>"
)

// OperatingSystems are the legal keys of path.default.
var OperatingSystems = []string{"macos", "linux"}

type Entry struct {
	Kind       Kind    `toml:"kind"`
	Name       string  `toml:"name"`
	Executable string  `toml:"executable"`
	Path       string  `toml:"path"`
	Places     []Place `toml:"places"`
}

type Place struct {
	Name     string    `toml:"name"`
	Holds    Holds     `toml:"holds"`
	Requires *Requires `toml:"requires"`
	Path     *Path     `toml:"path"`
	List     *List     `toml:"list"`
	Measure  Measure   `toml:"measure"`
	Clean    Clean     `toml:"clean"`
}

type Requires struct {
	Command string `toml:"command"`
}

type Path struct {
	Command string            `toml:"command"`
	Env     string            `toml:"env"`
	Subdir  string            `toml:"subdir"`
	Default map[string]string `toml:"default"`
}

type List struct {
	Command string `toml:"command"`
	Pattern string `toml:"pattern"`
}

type Measure struct {
	By      MeasureBy `toml:"by"`
	Path    string    `toml:"path"`
	Command string    `toml:"command"`
	Pattern string    `toml:"pattern"`
}

type Clean struct {
	Before  string `toml:"before"`
	Command string `toml:"command"`
}
