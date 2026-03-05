package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/xuri/excelize/v2"
)

type ImportHandler struct {
	q *sqlcdb.Queries
}

func NewImportHandler(q *sqlcdb.Queries) *ImportHandler {
	return &ImportHandler{q: q}
}

type importCruiseRow struct {
	Name          string   `json:"name"`
	Year          *int64   `json:"year"`
	EmbarkDate    *string  `json:"embark_date"`
	DisembarkDate *string  `json:"disembark_date"`
	Countries     *string  `json:"countries"`
	StartPort     *string  `json:"start_port"`
	EndPort       *string  `json:"end_port"`
	HoursTotal    *float64 `json:"hours_total"`
	HoursSail     *float64 `json:"hours_sail"`
	HoursEngine   *float64 `json:"hours_engine"`
	HoursOver6bf  *float64 `json:"hours_over_6bf"`
	Miles         *float64 `json:"miles"`
	Days          *int64   `json:"days"`
	CaptainName   *string  `json:"captain_name"`
	YachtName     *string  `json:"yacht_name"`
	YachtType     *string  `json:"yacht_type"`
	TidalWaters   *int64   `json:"tidal_waters"`
	CostTotal     *float64 `json:"cost_total"`
	CostPerPerson *float64 `json:"cost_per_person"`
	Description   *string  `json:"description"`
}

type importTrainingRow struct {
	Date      *string  `json:"date"`
	Name      string   `json:"name"`
	Organizer *string  `json:"organizer"`
	Cost      *float64 `json:"cost"`
	Url       *string  `json:"url"`
}

type importPreview struct {
	Cruises   []importCruiseRow   `json:"cruises"`
	Trainings []importTrainingRow `json:"trainings"`
}

type importConfirmRequest struct {
	Cruises   []importCruiseRow   `json:"cruises"`
	Trainings []importTrainingRow `json:"trainings"`
}

type importConfirmResult struct {
	CruisesCreated   int `json:"cruises_created"`
	TrainingsCreated int `json:"trainings_created"`
	YachtsCreated    int `json:"yachts_created"`
	CrewCreated      int `json:"crew_created"`
}

func (h *ImportHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("failed to close uploaded file: %v", err)
		}
	}()

	f, err := excelize.OpenReader(file)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse xlsx file")
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("failed to close xlsx file: %v", err)
		}
	}()

	cruises, err := parseOpinionSheet(f)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse opinie sheet: %v", err))
		return
	}

	trainings, err := parseTrainingSheet(f)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse szkolenia sheet: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, importPreview{
		Cruises:   cruises,
		Trainings: trainings,
	})
}

func (h *ImportHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req importConfirmRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	yachtIDs := map[string]int64{}
	yachtsCreated := 0
	captainIDs := map[string]int64{}
	crewCreated := 0

	for _, c := range req.Cruises {
		if c.YachtName != nil && *c.YachtName != "" {
			name := *c.YachtName
			if _, ok := yachtIDs[name]; !ok {
				existing, err := h.q.GetYachtByName(r.Context(), sqlcdb.GetYachtByNameParams{
					OwnerID: user.UserID,
					Name:    name,
				})
				if err == nil {
					yachtIDs[name] = existing.ID
				} else {
					yachtType := sql.NullString{}
					if c.YachtType != nil {
						yachtType = sql.NullString{String: *c.YachtType, Valid: true}
					}
					created, err := h.q.CreateYacht(r.Context(), sqlcdb.CreateYachtParams{
						OwnerID:   user.UserID,
						Name:      name,
						YachtType: yachtType,
					})
					if err != nil {
						respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create yacht %q: %v", name, err))
						return
					}
					yachtIDs[name] = created.ID
					yachtsCreated++
				}
			}
		}

		if c.CaptainName != nil && *c.CaptainName != "" {
			name := *c.CaptainName
			if _, ok := captainIDs[name]; !ok {
				existing, err := h.q.GetCrewMemberByName(r.Context(), sqlcdb.GetCrewMemberByNameParams{
					OwnerID:  user.UserID,
					FullName: name,
				})
				if err == nil {
					captainIDs[name] = existing.ID
				} else {
					created, err := h.q.CreateCrewMember(r.Context(), sqlcdb.CreateCrewMemberParams{
						OwnerID:  user.UserID,
						FullName: name,
					})
					if err != nil {
						respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create crew member %q: %v", name, err))
						return
					}
					captainIDs[name] = created.ID
					crewCreated++
				}
			}
		}
	}

	cruisesCreated := 0
	for _, c := range req.Cruises {
		if c.Name == "" {
			continue
		}
		var yachtID sql.NullInt64
		if c.YachtName != nil && *c.YachtName != "" {
			yachtID = sql.NullInt64{Int64: yachtIDs[*c.YachtName], Valid: true}
		}
		var tidalWaters sql.NullInt64
		if c.TidalWaters != nil {
			tidalWaters = sql.NullInt64{Int64: *c.TidalWaters, Valid: true}
		}

		_, err := h.q.CreateCruise(r.Context(), sqlcdb.CreateCruiseParams{
			OwnerID:       user.UserID,
			Name:          c.Name,
			Year:          nullInt64(c.Year),
			EmbarkDate:    nullString(c.EmbarkDate),
			DisembarkDate: nullString(c.DisembarkDate),
			Countries:     nullString(c.Countries),
			StartPort:     nullString(c.StartPort),
			EndPort:       nullString(c.EndPort),
			HoursTotal:    nullFloat64(c.HoursTotal),
			HoursSail:     nullFloat64(c.HoursSail),
			HoursEngine:   nullFloat64(c.HoursEngine),
			HoursOver6bf:  nullFloat64(c.HoursOver6bf),
			Miles:         nullFloat64(c.Miles),
			Days:          nullInt64(c.Days),
			CaptainName:   nullString(c.CaptainName),
			YachtID:       yachtID,
			TidalWaters:   tidalWaters,
			CostTotal:     nullFloat64(c.CostTotal),
			CostPerPerson: nullFloat64(c.CostPerPerson),
			Description:   nullString(c.Description),
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create cruise %q: %v", c.Name, err))
			return
		}
		cruisesCreated++
	}

	trainingsCreated := 0
	for _, t := range req.Trainings {
		if t.Name == "" {
			continue
		}
		_, err := h.q.CreateTraining(r.Context(), sqlcdb.CreateTrainingParams{
			UserID:    user.UserID,
			Date:      nullString(t.Date),
			Name:      t.Name,
			Organizer: nullString(t.Organizer),
			Cost:      nullFloat64(t.Cost),
			Url:       nullString(t.Url),
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create training %q: %v", t.Name, err))
			return
		}
		trainingsCreated++
	}

	respondJSON(w, http.StatusCreated, importConfirmResult{
		CruisesCreated:   cruisesCreated,
		TrainingsCreated: trainingsCreated,
		YachtsCreated:    yachtsCreated,
		CrewCreated:      crewCreated,
	})
}

func parseOpinionSheet(f *excelize.File) ([]importCruiseRow, error) {
	sheetName := "opinie"
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("sheet %q not found: %w", sheetName, err)
	}

	var cruises []importCruiseRow
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || strings.TrimSpace(cellAt(row, 0)) == "" {
			continue
		}

		cruise := importCruiseRow{
			Name:          strings.TrimSpace(cellAt(row, 0)),
			Year:          parseInt64(cellAt(row, 1)),
			EmbarkDate:    parseExcelDate(f, sheetName, i+1, 2),
			DisembarkDate: parseExcelDate(f, sheetName, i+1, 3),
			Countries:     optString(cellAt(row, 4)),
			StartPort:     optString(cellAt(row, 5)),
			EndPort:       optString(cellAt(row, 6)),
			HoursTotal:    parseFloat(cellAt(row, 7)),
			HoursSail:     parseFloat(cellAt(row, 8)),
			HoursEngine:   parseFloat(cellAt(row, 9)),
			HoursOver6bf:  parseFloat(cellAt(row, 10)),
			Miles:         parseFloat(cellAt(row, 11)),
			Days:          parseInt64(cellAt(row, 12)),
			CaptainName:   optString(cellAt(row, 13)),
			YachtName:     optString(cellAt(row, 14)),
			YachtType:     optString(cellAt(row, 15)),
			TidalWaters:   parseTidalWaters(cellAt(row, 16)),
			CostTotal:     parseFloat(cellAt(row, 17)),
			CostPerPerson: parseFloat(cellAt(row, 18)),
			Description:   optString(cellAt(row, 19)),
		}
		cruises = append(cruises, cruise)
	}
	return cruises, nil
}

func parseTrainingSheet(f *excelize.File) ([]importTrainingRow, error) {
	sheetName := "szkolenia"
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("sheet %q not found: %w", sheetName, err)
	}

	var trainings []importTrainingRow
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || strings.TrimSpace(cellAt(row, 0)) == "" {
			continue
		}

		training := importTrainingRow{
			Date:      optString(cellAt(row, 0)),
			Name:      strings.TrimSpace(cellAt(row, 1)),
			Organizer: optString(cellAt(row, 2)),
			Cost:      parseFloat(cellAt(row, 3)),
			Url:       optString(cellAt(row, 4)),
		}
		trainings = append(trainings, training)
	}
	return trainings, nil
}

func cellAt(row []string, idx int) string {
	if idx >= len(row) {
		return ""
	}
	return row[idx]
}

func optString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func parseInt64(s string) *int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	v := int64(f)
	return &v
}

func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseTidalWaters(s string) *int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return nil
	}
	var v int64
	if s == "tak" {
		v = 1
	}
	return &v
}

func parseExcelDate(f *excelize.File, sheet string, rowIdx, colIdx int) *string {
	colName, err := excelize.ColumnNumberToName(colIdx + 1)
	if err != nil {
		return nil
	}
	cell := fmt.Sprintf("%s%d", colName, rowIdx)

	raw, err := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}

	serial, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		formatted, _ := f.GetCellValue(sheet, cell)
		return optString(formatted)
	}

	t := excelSerialToTime(serial)
	s := t.Format(time.DateOnly)
	return &s
}

func excelSerialToTime(serial float64) time.Time {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	days := int(math.Floor(serial))
	fraction := serial - float64(days)
	secs := int(math.Round(fraction * 86400))
	return epoch.AddDate(0, 0, days).Add(time.Duration(secs) * time.Second)
}
