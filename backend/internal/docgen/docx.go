package docgen

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/lukasjarosch/go-docx"
)

//go:embed templates/voyage_opinion.docx
var docxTemplate []byte

func GenerateDOCX(data OpinionData) ([]byte, error) {
	doc, err := docx.OpenBytes(docxTemplate)
	if err != nil {
		return nil, fmt.Errorf("open docx template: %w", err)
	}

	tidalStr := "Nie / No"
	if data.TidalWaters {
		tidalStr = "Tak / Yes"
	}

	placeholders := docx.PlaceholderMap{
		"CrewMemberName": data.CrewMemberName,
		"PatentNumber":   data.PatentNumber,
		"Role":           data.Role,
		"CruiseName":     data.CruiseName,
		"YachtName":      data.YachtName,
		"YachtType":      data.YachtType,
		"EmbarkDate":     data.EmbarkDate,
		"DisembarkDate":  data.DisembarkDate,
		"StartPort":      data.StartPort,
		"EndPort":        data.EndPort,
		"Countries":      data.Countries,
		"Miles":          fmt.Sprintf("%.1f", data.Miles),
		"Days":           fmt.Sprintf("%d", data.Days),
		"HoursTotal":     fmt.Sprintf("%.1f", data.HoursTotal),
		"HoursSail":      fmt.Sprintf("%.1f", data.HoursSail),
		"HoursEngine":    fmt.Sprintf("%.1f", data.HoursEngine),
		"HoursOver6bf":   fmt.Sprintf("%.1f", data.HoursOver6bf),
		"TidalWaters":    tidalStr,
		"CaptainName":    data.CaptainName,
		"GeneratedDate":  data.GeneratedDate,
	}

	if err := doc.ReplaceAll(placeholders); err != nil {
		return nil, fmt.Errorf("replace placeholders: %w", err)
	}

	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		return nil, fmt.Errorf("write docx: %w", err)
	}

	return buf.Bytes(), nil
}
