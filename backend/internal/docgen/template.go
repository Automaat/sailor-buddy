package docgen

import (
	"bytes"
	_ "embed"
	"html/template"
)

type OpinionData struct {
	CrewMemberName string
	PatentNumber   string
	CruiseName     string
	EmbarkDate     string
	DisembarkDate  string
	YachtName      string
	YachtType      string
	StartPort      string
	EndPort        string
	Countries      string
	Miles          float64
	HoursTotal     float64
	HoursSail      float64
	HoursEngine    float64
	HoursOver6bf   float64
	Days           int64
	TidalWaters    bool
	CaptainName    string
	Role           string
	GeneratedDate  string
}

//go:embed templates/voyage_opinion.html
var htmlTemplate string

var tmpl = template.Must(template.New("opinion").Parse(htmlTemplate))

func RenderHTML(data OpinionData) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
