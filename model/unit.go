package model

const (
	APP    UnitType = 1 // App always overrides all
	MODULE UnitType = 2 // Module is next
	REVEL  UnitType = 3 // Revel is last
)

type (
	Unit struct {
		Name       string   // The friendly name for the unit
		Config     string   // The config file contents
		Type       UnitType // The type of the unit
		Messages   string   // The messages
		BasePath   string   // The filesystem path of the unit
		ImportPath string   // The import path for the package
		Container  *Container
	}
	UnitList []*Unit
	UnitType int
)
