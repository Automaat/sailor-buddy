package docgen

import (
	"strings"
	"testing"
)

func testData() OpinionData {
	return OpinionData{
		CrewMemberName: "Jan Kowalski",
		PatentNumber:   "PL-12345",
		CruiseName:     "Adriatyk 2025",
		EmbarkDate:     "2025-07-01",
		DisembarkDate:  "2025-07-14",
		YachtName:      "SY Orion",
		YachtType:      "s/y",
		StartPort:      "Split",
		EndPort:        "Dubrovnik",
		Countries:      "Chorwacja",
		Miles:          250.5,
		HoursTotal:     120,
		HoursSail:      80,
		HoursEngine:    40,
		HoursOver6bf:   10,
		Days:           14,
		TidalWaters:    false,
		CaptainName:    "Adam Nowak",
		Role:           "Sternik",
		GeneratedDate:  "2025-08-01",
	}
}

func TestRenderHTML(t *testing.T) {
	html, err := RenderHTML(testData())
	if err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}

	content := string(html)
	checks := []string{
		"Jan Kowalski",
		"PL-12345",
		"Adriatyk 2025",
		"Split",
		"Dubrovnik",
		"250.5",
		"Adam Nowak",
		"Nie / No",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("expected HTML to contain %q", check)
		}
	}
}

func TestRenderHTML_TidalWaters(t *testing.T) {
	data := testData()
	data.TidalWaters = true
	html, err := RenderHTML(data)
	if err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}
	if !strings.Contains(string(html), "Tak / Yes") {
		t.Error("expected tidal waters = Yes")
	}
}

func TestGenerateDOCX(t *testing.T) {
	out, err := GenerateDOCX(testData())
	if err != nil {
		t.Fatalf("GenerateDOCX failed: %v", err)
	}
	if len(out) < 100 {
		t.Fatal("DOCX output too small")
	}
	// DOCX files start with PK (zip header)
	if out[0] != 'P' || out[1] != 'K' {
		t.Fatal("output is not a valid zip/docx file")
	}
}
