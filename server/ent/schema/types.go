package schema

// PlotState is stored as JSONB in farms.plots (8×8 grid).
type PlotState struct {
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Type       string `json:"type"`       // empty/tilled/planted/mature/withered
	CropID     string `json:"cropId,omitempty"`
	PlantedAt  string `json:"plantedAt,omitempty"`
	Stage      string `json:"stage,omitempty"`    // seedling/growing/mature
	Quality    string `json:"quality,omitempty"`  // normal/good/excellent
	WateredAt  string `json:"wateredAt,omitempty"`
	Fertilized bool   `json:"fertilized"`
}

// RequirementItem is one element in village_projects.requirements JSONB array.
type RequirementItem struct {
	Type     string `json:"type"`              // material/coins
	ItemID   string `json:"itemId,omitempty"`
	Required int64  `json:"required"`
	Current  int64  `json:"current"`
}
