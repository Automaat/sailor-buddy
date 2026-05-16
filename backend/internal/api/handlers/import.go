package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/xuri/excelize/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type ImportHandler struct {
	runTx txRunner
}

func NewImportHandler(db *sql.DB) *ImportHandler {
	return &ImportHandler{runTx: sqlTxRunner(db)}
}

// importCruiseRow and importTrainingRow alias the DTO rows so the spreadsheet
// parsing helpers and the huma handlers share one type.
type (
	importCruiseRow   = dto.ImportVoyageRow
	importTrainingRow = dto.ImportTrainingRow
)

type importUploadInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true" doc:"XLSX spreadsheet to import"`
	}]
}

type importPreviewOutput struct {
	Body dto.ImportData
}

type importConfirmInput struct {
	Body dto.ImportData
}

type importResultOutput struct {
	Body dto.ImportResult
}

// RegisterImportRoutes wires the XLSX import operations onto the API.
func RegisterImportRoutes(api huma.API, db *sql.DB) {
	NewImportHandler(db).registerRoutes(api)
}

func (h *ImportHandler) registerRoutes(api huma.API) {
	tag := []string{"Import"}

	huma.Register(api, huma.Operation{
		OperationID: "import-xlsx", Method: http.MethodPost, Path: "/import/xlsx",
		Summary: "Upload an XLSX file and preview the parsed records", Tags: tag,
	}, h.upload)
	huma.Register(api, huma.Operation{
		OperationID: "import-confirm", Method: http.MethodPost, Path: "/import/confirm",
		Summary: "Persist the reviewed import preview", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.confirm)
}

func (h *ImportHandler) upload(_ context.Context, in *importUploadInput) (*importPreviewOutput, error) {
	file := in.RawBody.Data().File
	if !file.IsSet {
		return nil, huma.Error400BadRequest("missing file field")
	}
	defer func() { _ = file.Close() }()

	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, huma.Error400BadRequest("failed to parse xlsx file")
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Error("close xlsx file", "err", cerr)
		}
	}()

	cruises, err := parseOpinionSheet(f)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("failed to parse opinie sheet: %v", err))
	}
	trainings, err := parseTrainingSheet(f)
	if err != nil {
		return nil, huma.Error400BadRequest(fmt.Sprintf("failed to parse szkolenia sheet: %v", err))
	}
	return &importPreviewOutput{Body: dto.ImportData{Voyages: cruises, Trainings: trainings}}, nil
}

func (h *ImportHandler) confirm(ctx context.Context, in *importConfirmInput) (*importResultOutput, error) {
	user := middleware.GetUser(ctx)

	var result dto.ImportResult
	// Resolve entities and create voyages and trainings in one transaction:
	// a failure on any step rolls back the whole spreadsheet rather than
	// leaving a partial import.
	err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		yachtIDs, yachtsCreated, crewCreated, rerr := resolveImportEntities(ctx, q, user.UserID, in.Body.Voyages)
		if rerr != nil {
			return rerr
		}
		voyagesCreated, rerr := createImportVoyages(ctx, q, user.UserID, in.Body.Voyages, yachtIDs)
		if rerr != nil {
			return rerr
		}
		trainingsCreated, rerr := createImportTrainings(ctx, q, user.UserID, in.Body.Trainings)
		if rerr != nil {
			return rerr
		}
		result = dto.ImportResult{
			VoyagesCreated:   voyagesCreated,
			TrainingsCreated: trainingsCreated,
			YachtsCreated:    yachtsCreated,
			CrewCreated:      crewCreated,
		}
		return nil
	})
	if err != nil {
		slog.Error("import confirm", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &importResultOutput{Body: result}, nil
}

// resolveImportEntities ensures every yacht and captain referenced by the
// imported voyages exists, creating any that are missing. It returns the
// name->ID map for yachts plus counts of newly created yachts and crew.
func resolveImportEntities(ctx context.Context, q sqlcdb.Querier, ownerID int64, voyages []importCruiseRow) (yachtIDs map[string]int64, yachtsCreated, crewCreated int, err error) {
	yachtIDs = map[string]int64{}
	captainIDs := map[string]int64{}

	for i := range voyages {
		c := &voyages[i]
		if c.YachtName != nil && *c.YachtName != "" {
			created, rerr := resolveYacht(ctx, q, ownerID, yachtIDs, *c.YachtName, c.YachtType)
			if rerr != nil {
				return nil, 0, 0, rerr
			}
			if created {
				yachtsCreated++
			}
		}
		if c.CaptainName != nil && *c.CaptainName != "" {
			created, rerr := resolveCaptain(ctx, q, ownerID, captainIDs, *c.CaptainName)
			if rerr != nil {
				return nil, 0, 0, rerr
			}
			if created {
				crewCreated++
			}
		}
	}
	return yachtIDs, yachtsCreated, crewCreated, nil
}

func resolveYacht(ctx context.Context, q sqlcdb.Querier, ownerID int64, yachtIDs map[string]int64, name string, yachtTypePtr *string) (created bool, err error) {
	if _, ok := yachtIDs[name]; ok {
		return false, nil
	}
	existing, lookupErr := q.GetYachtByName(ctx, sqlcdb.GetYachtByNameParams{OwnerID: ownerID, Name: name})
	if lookupErr == nil {
		yachtIDs[name] = existing.ID
		return false, nil
	}
	yachtType := types.NullString{}
	if yachtTypePtr != nil {
		yachtType = types.NullString{String: *yachtTypePtr, Valid: true}
	}
	row, createErr := q.CreateYacht(ctx, sqlcdb.CreateYachtParams{
		OwnerID:   ownerID,
		Name:      name,
		YachtType: yachtType,
	})
	if createErr != nil {
		return false, fmt.Errorf("failed to create yacht %q: %w", name, createErr)
	}
	yachtIDs[name] = row.ID
	return true, nil
}

func resolveCaptain(ctx context.Context, q sqlcdb.Querier, ownerID int64, captainIDs map[string]int64, name string) (created bool, err error) {
	if _, ok := captainIDs[name]; ok {
		return false, nil
	}
	existing, lookupErr := q.GetCrewMemberByName(ctx, sqlcdb.GetCrewMemberByNameParams{OwnerID: ownerID, FullName: name})
	if lookupErr == nil {
		captainIDs[name] = existing.ID
		return false, nil
	}
	row, createErr := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{OwnerID: ownerID, FullName: name})
	if createErr != nil {
		return false, fmt.Errorf("failed to create crew member %q: %w", name, createErr)
	}
	captainIDs[name] = row.ID
	return true, nil
}

func createImportVoyages(ctx context.Context, q sqlcdb.Querier, ownerID int64, voyages []importCruiseRow, yachtIDs map[string]int64) (int, error) {
	created := 0
	for i := range voyages {
		c := &voyages[i]
		if c.Name == "" {
			continue
		}
		var yachtID types.NullInt64
		if c.YachtName != nil && *c.YachtName != "" {
			yachtID = types.NullInt64{Int64: yachtIDs[*c.YachtName], Valid: true}
		}
		_, err := q.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
			OwnerID:       ownerID,
			Name:          c.Name,
			Year:          nullInt64(c.Year),
			EmbarkDate:    nullString(c.EmbarkDate),
			DisembarkDate: nullString(c.DisembarkDate),
			Countries:     nullString(c.Countries),
			StartPort:     nullString(c.StartPort),
			EndPort:       nullString(c.EndPort),
			HoursTotal:    valOrZeroFloat(c.HoursTotal),
			HoursSail:     valOrZeroFloat(c.HoursSail),
			HoursEngine:   valOrZeroFloat(c.HoursEngine),
			HoursOver6bf:  valOrZeroFloat(c.HoursOver6bf),
			Miles:         valOrZeroFloat(c.Miles),
			Days:          valOrZeroInt(c.Days),
			CaptainName:   nullString(c.CaptainName),
			YachtID:       yachtID,
			TidalWaters:   valOrZeroInt(c.TidalWaters),
			CostTotal:     nullFloat64(c.CostTotal),
			CostPerPerson: nullFloat64(c.CostPerPerson),
			Description:   nullString(c.Description),
		})
		if err != nil {
			return created, fmt.Errorf("failed to create voyage %q: %w", c.Name, err)
		}
		created++
	}
	return created, nil
}

func createImportTrainings(ctx context.Context, q sqlcdb.Querier, userID int64, trainings []importTrainingRow) (int, error) {
	created := 0
	for i := range trainings {
		t := &trainings[i]
		if t.Name == "" {
			continue
		}
		_, err := q.CreateTraining(ctx, sqlcdb.CreateTrainingParams{
			UserID:    userID,
			Date:      nullString(t.Date),
			Name:      t.Name,
			Organizer: nullString(t.Organizer),
			Cost:      nullFloat64(t.Cost),
			Url:       nullString(t.Url),
		})
		if err != nil {
			return created, fmt.Errorf("failed to create training %q: %w", t.Name, err)
		}
		created++
	}
	return created, nil
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

		cruises = append(cruises, importCruiseRow{
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
		})
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

		trainings = append(trainings, importTrainingRow{
			Date:      optString(cellAt(row, 0)),
			Name:      strings.TrimSpace(cellAt(row, 1)),
			Organizer: optString(cellAt(row, 2)),
			Cost:      parseFloat(cellAt(row, 3)),
			Url:       optString(cellAt(row, 4)),
		})
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
