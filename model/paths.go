package model

type Paths struct {
	Import    string
	Source    string
	Base      string
	Code      []string         // Consolidated code paths
	Template  []string         // Consolidated template paths
	Config    []string         // Consolidated configuration paths
	ModuleMap map[string]*Unit // The module path map
}
