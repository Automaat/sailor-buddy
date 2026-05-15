package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// VoyageOpinion is a generated crew opinion document. The on-disk file path is
// intentionally not exposed; the file is retrieved via the download endpoint.
type VoyageOpinion struct {
	ID           int64     `json:"id"`
	VoyageID     int64     `json:"voyage_id"`
	CrewMemberID int64     `json:"crew_member_id"`
	FileFormat   string    `json:"file_format"`
	FullName     string    `json:"full_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// GenerateOpinionBody is the request payload for generating an opinion.
type GenerateOpinionBody struct {
	CrewMemberID int64  `json:"crew_member_id" doc:"Crew member ID"`
	Format       string `json:"format,omitempty" enum:"pdf,docx" default:"pdf" doc:"Document format"`
}

// VoyageOpinionFromDB maps an opinion row to the API model.
func VoyageOpinionFromDB(o sqlcdb.VoyageOpinion) VoyageOpinion {
	return VoyageOpinion{
		ID:           o.ID,
		VoyageID:     o.VoyageID,
		CrewMemberID: o.CrewMemberID,
		FileFormat:   o.FileFormat,
		CreatedAt:    timeVal(o.CreatedAt),
	}
}

// VoyageOpinionsFromDB maps the joined opinion rows, returning a non-nil slice.
func VoyageOpinionsFromDB(rows []sqlcdb.ListVoyageVoyageOpinionsRow) []VoyageOpinion {
	out := make([]VoyageOpinion, len(rows))
	for i := range rows {
		out[i] = VoyageOpinion{
			ID:           rows[i].ID,
			VoyageID:     rows[i].VoyageID,
			CrewMemberID: rows[i].CrewMemberID,
			FileFormat:   rows[i].FileFormat,
			FullName:     rows[i].FullName,
			CreatedAt:    timeVal(rows[i].CreatedAt),
		}
	}
	return out
}
