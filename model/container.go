package model

// The single instance object that has the config populated to it
type (
	Container struct {
		Controller Controller
		Paths      Paths
	}
)
