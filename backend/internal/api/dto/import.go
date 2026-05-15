package dto

// ImportVoyageRow is one parsed voyage row from an imported spreadsheet.
type ImportVoyageRow struct {
	Name          string   `json:"name"`
	Year          *int64   `json:"year,omitempty"`
	EmbarkDate    *string  `json:"embark_date,omitempty"`
	DisembarkDate *string  `json:"disembark_date,omitempty"`
	Countries     *string  `json:"countries,omitempty"`
	StartPort     *string  `json:"start_port,omitempty"`
	EndPort       *string  `json:"end_port,omitempty"`
	HoursTotal    *float64 `json:"hours_total,omitempty"`
	HoursSail     *float64 `json:"hours_sail,omitempty"`
	HoursEngine   *float64 `json:"hours_engine,omitempty"`
	HoursOver6bf  *float64 `json:"hours_over_6bf,omitempty"`
	Miles         *float64 `json:"miles,omitempty"`
	Days          *int64   `json:"days,omitempty"`
	CaptainName   *string  `json:"captain_name,omitempty"`
	YachtName     *string  `json:"yacht_name,omitempty"`
	YachtType     *string  `json:"yacht_type,omitempty"`
	TidalWaters   *int64   `json:"tidal_waters,omitempty"`
	CostTotal     *float64 `json:"cost_total,omitempty"`
	CostPerPerson *float64 `json:"cost_per_person,omitempty"`
	Description   *string  `json:"description,omitempty"`
}

// ImportTrainingRow is one parsed training row from an imported spreadsheet.
type ImportTrainingRow struct {
	Date      *string  `json:"date,omitempty"`
	Name      string   `json:"name"`
	Organizer *string  `json:"organizer,omitempty"`
	Cost      *float64 `json:"cost,omitempty"`
	Url       *string  `json:"url,omitempty"`
}

// ImportData is the parsed-and-reviewed import payload, used both as the
// preview response and the confirm request.
type ImportData struct {
	Voyages   []ImportVoyageRow   `json:"voyages"`
	Trainings []ImportTrainingRow `json:"trainings"`
}

// ImportResult reports how many records each import created.
type ImportResult struct {
	VoyagesCreated   int `json:"voyages_created"`
	TrainingsCreated int `json:"trainings_created"`
	YachtsCreated    int `json:"yachts_created"`
	CrewCreated      int `json:"crew_created"`
}
