package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/rangetable"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type CruiseHandler struct {
	q sqlcdb.Querier
}

func NewCruiseHandler(q sqlcdb.Querier) *CruiseHandler {
	return &CruiseHandler{q: q}
}

type cruiseIDParam struct {
	ID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type createCruiseInput struct {
	Body dto.CruiseBody
}

type updateCruiseInput struct {
	ID   int64 `path:"cruiseID" doc:"Cruise ID"`
	Body dto.CruiseBody
}

type cruiseOutput struct {
	Body dto.Cruise
}

type cruiseListOutput struct {
	Body dto.Page[dto.Cruise]
}

type cruiseEnrollmentsParam struct {
	CruiseID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type cruiseEnrollmentParam struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
}

type cruiseEnrollmentStatusInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.EnrollmentStatusBody
}

type cruiseEnrollmentTripInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.AssignTripBody
}

type cruiseEnrollmentsOutput struct {
	Body []dto.CruiseEnrollmentDetail
}

// RegisterCruiseRoutes wires the club cruise operations onto the API. Reads are
// open to any member; mutations require an admin.
func RegisterCruiseRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewCruiseHandler(q)
	tag := []string{"Cruises"}

	huma.Register(api, huma.Operation{
		OperationID: "list-cruises", Method: http.MethodGet, Path: "/cruises",
		Summary: "List cruises", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-cruise", Method: http.MethodGet, Path: "/cruises/{cruiseID}",
		Summary: "Get a cruise", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-cruise", Method: http.MethodPost, Path: "/cruises",
		Summary: "Create a cruise (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise", Method: http.MethodPut, Path: "/cruises/{cruiseID}",
		Summary: "Update a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise", Method: http.MethodDelete, Path: "/cruises/{cruiseID}",
		Summary: "Delete a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "generate-cruise-enroll-token", Method: http.MethodPost, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Generate a cruise enrollment token (admin)", Tags: tag,
	}, h.generateEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-cruise-enroll-token", Method: http.MethodDelete, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Clear a cruise enrollment token (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-trips", Method: http.MethodGet, Path: "/cruises/{cruiseID}/trips",
		Summary: "List a cruise's child trips", Tags: tag,
	}, h.listChildTrips)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-voyages", Method: http.MethodGet, Path: "/cruises/{cruiseID}/voyages",
		Summary: "List a cruise's child voyages", Tags: tag,
	}, h.listChildVoyages)

	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-enrollments", Method: http.MethodGet,
		Path:    "/cruises/{cruiseID}/enrollments",
		Summary: "List a cruise's enrollments (admin)", Tags: tag,
	}, h.listEnrollments)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise-enrollment-status", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/status",
		Summary: "Update a cruise enrollment's status (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateEnrollmentStatus)
	huma.Register(api, huma.Operation{
		OperationID: "assign-cruise-enrollment-trip", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/trip",
		Summary: "Assign a cruise enrollment to a trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.assignEnrollmentToTrip)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise-enrollment", Method: http.MethodDelete,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}",
		Summary: "Delete a cruise enrollment (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteEnrollment)
}

func (h *CruiseHandler) list(ctx context.Context, in *pageParams) (*cruiseListOutput, error) {
	cruises, err := h.q.ListCruises(ctx, sqlcdb.ListCruisesParams{Limit: in.Limit, Offset: in.Offset})
	if err != nil {
		slog.Error("list cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	total, err := h.q.CountCruises(ctx)
	if err != nil {
		slog.Error("count cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	return &cruiseListOutput{Body: dto.NewPage(dto.CruisesFromDB(cruises), total, in.Limit, in.Offset)}, nil
}

func (h *CruiseHandler) get(ctx context.Context, in *cruiseIDParam) (*cruiseOutput, error) {
	cruise, err := h.q.GetCruise(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("get cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) create(ctx context.Context, in *createCruiseInput) (*cruiseOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	seriesID, err := h.resolveSeries(ctx, in.Body.SeriesID)
	if err != nil {
		return nil, err
	}
	included, notIncluded, err := cruisePriceLists(&in.Body)
	if err != nil {
		return nil, err
	}
	chatURL, err := groupChatURL(in.Body.GroupChatUrl)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	cruise, err := h.q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		CreatedBy:              types.NullInt64{Int64: user.UserID, Valid: true},
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		Description:            nullString(in.Body.Description),
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		MaxCrew:                nullInt64(in.Body.MaxCrew),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Kind:                   in.Body.Kind,
		SeriesID:               seriesID,
		Edition:                nullEdition(in.Body.Edition),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		Included:               included,
		NotIncluded:            notIncluded,
		GroupChatUrl:           chatURL,
		TravelNotes:            nullTrimmed(in.Body.TravelNotes),
		MinAge:                 nullInt64(in.Body.MinAge),
	})
	if err != nil {
		if isForeignKeyViolation(err, cruiseSeriesFKey) {
			return nil, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("create cruise", "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) update(ctx context.Context, in *updateCruiseInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	seriesID, err := h.resolveSeries(ctx, in.Body.SeriesID)
	if err != nil {
		return nil, err
	}
	included, notIncluded, err := cruisePriceLists(&in.Body)
	if err != nil {
		return nil, err
	}
	chatURL, err := groupChatURL(in.Body.GroupChatUrl)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruise(ctx, sqlcdb.UpdateCruiseParams{
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		Description:            nullString(in.Body.Description),
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		MaxCrew:                nullInt64(in.Body.MaxCrew),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Kind:                   in.Body.Kind,
		SeriesID:               seriesID,
		Edition:                nullEdition(in.Body.Edition),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		Included:               included,
		NotIncluded:            notIncluded,
		GroupChatUrl:           chatURL,
		TravelNotes:            nullTrimmed(in.Body.TravelNotes),
		MinAge:                 nullInt64(in.Body.MinAge),
		ID:                     in.ID,
	}); err != nil {
		if isForeignKeyViolation(err, cruiseSeriesFKey) {
			return nil, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("update cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) delete(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruise(ctx, in.ID); err != nil {
		if isForeignKeyViolation(err, "trips_cruise_id_fkey") || isForeignKeyViolation(err, "voyages_cruise_id_fkey") {
			return nil, huma.Error409Conflict("cruise still has trips or voyages; delete them first")
		}
		slog.Error("delete cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) generateEnrollToken(ctx context.Context, in *cruiseIDParam) (*tokenOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)
	if err := h.q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
	}); err != nil {
		slog.Error("set cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *CruiseHandler) clearEnrollToken(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.ClearCruiseEnrollToken(ctx, in.ID); err != nil {
		slog.Error("clear cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) listChildTrips(ctx context.Context, in *cruiseIDParam) (*cruiseTripsOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	trips, err := h.q.ListCruiseTrips(ctx, in.ID)
	if err != nil {
		slog.Error("list cruise child trips", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &cruiseTripsOutput{Body: dto.TripsFromDB(trips)}, nil
}

func (h *CruiseHandler) listChildVoyages(ctx context.Context, in *cruiseIDParam) (*cruiseVoyagesOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	voyages, err := h.q.ListCruiseVoyages(ctx, in.ID)
	if err != nil {
		slog.Error("list cruise child voyages", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &cruiseVoyagesOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

func (h *CruiseHandler) listEnrollments(ctx context.Context, in *cruiseEnrollmentsParam) (*cruiseEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	enrollments, err := h.q.ListCruiseEnrollments(ctx, in.CruiseID)
	if err != nil {
		slog.Error("list cruise enrollments", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &cruiseEnrollmentsOutput{Body: dto.CruiseEnrollmentsFromDB(enrollments)}, nil
}

func (h *CruiseHandler) updateEnrollmentStatus(ctx context.Context, in *cruiseEnrollmentStatusInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruiseEnrollmentStatus(ctx, sqlcdb.UpdateCruiseEnrollmentStatusParams{
		Status: in.Body.Status,
		ID:     in.EnrollmentID,
	}); err != nil {
		slog.Error("update cruise enrollment status", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) assignEnrollmentToTrip(ctx context.Context, in *cruiseEnrollmentTripInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.AssignCruiseEnrollmentToTrip(ctx, sqlcdb.AssignCruiseEnrollmentToTripParams{
		TripID: nullInt64(in.Body.TripID),
		ID:     in.EnrollmentID,
	}); err != nil {
		slog.Error("assign cruise enrollment to trip", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign enrollment")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) deleteEnrollment(ctx context.Context, in *cruiseEnrollmentParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruiseEnrollment(ctx, in.EnrollmentID); err != nil {
		slog.Error("delete cruise enrollment", "enrollment_id", in.EnrollmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}

// verifyCruise confirms the cruise exists.
func (h *CruiseHandler) verifyCruise(ctx context.Context, cruiseID int64) error {
	if _, err := h.q.GetCruise(ctx, cruiseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise", "cruise_id", cruiseID, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}

// cruiseSeriesFKey is the PostgreSQL name of the cruises.series_id foreign key.
const cruiseSeriesFKey = "cruises_series_id_fkey"

// resolveSeries checks that an optional series exists. A missing series is a
// client error (422), not a server failure.
func (h *CruiseHandler) resolveSeries(ctx context.Context, id *int64) (types.NullInt64, error) {
	if id == nil {
		return types.NullInt64{}, nil
	}
	if _, err := h.q.GetCruiseSeries(ctx, *id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.NullInt64{}, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("verify series", "series_id", *id, "err", err)
		return types.NullInt64{}, huma.Error500InternalServerError("failed to verify series")
	}
	return types.NullInt64{Int64: *id, Valid: true}, nil
}

// nullEdition stores a blank edition label as null.
func nullEdition(s *string) types.NullString {
	return nullTrimmed(s)
}

// nullTrimmed trims surrounding whitespace and stores a blank value as null.
func nullTrimmed(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return types.NullString{}
	}
	return types.NullString{String: v, Valid: true}
}

// groupChatURL validates an optional group chat link. A blank link clears it;
// anything other than an absolute http or https URL is a client error (422).
func groupChatURL(s *string) (types.NullString, error) {
	v := nullTrimmed(s)
	if !v.Valid {
		return v, nil
	}
	u, err := url.Parse(v.String)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || !validPort(u.Port()) ||
		failsDomainToASCII(u.Hostname()) || invalidNumericHost(browserHost(u.Hostname())) {
		return types.NullString{}, huma.Error422UnprocessableEntity("group_chat_url must be an http or https URL")
	}
	return v, nil
}

// validPort accepts an absent port or one in 1-65535; url.Parse does not
// check the range, while browsers reject such links.
func validPort(p string) bool {
	if p == "" {
		return true
	}
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}

// whatwgIDNA approximates the WHATWG "domain to ASCII" profile: UTS46
// mapping without STD3, hyphen or DNS-length checks.
var whatwgIDNA = idna.New(
	idna.MapForLookup(),
	idna.Transitional(false),
	idna.StrictDomainName(false),
	idna.CheckHyphens(false),
	idna.VerifyDNSLength(false),
)

// browserHost maps host the way a browser does before its IPv4 check, so
// full-width digits or U+3002 dots read as ASCII. IDNA errors are not
// grounds for rejection here: idna may flag labels a browser accepts (e.g.
// newer Unicode), so its best-effort mapping is still used, and the raw
// host only when it yields nothing.
func browserHost(host string) string {
	mapped, _ := whatwgIDNA.ToASCII(host)
	if mapped == "" {
		return host
	}
	return mapped
}

// joinerIDNA runs only the UTS46 CheckJoiners (CONTEXTJ) rules: no mapping,
// no code point or bidi validation.
var joinerIDNA = idna.New(idna.CheckJoiners(true))

// assignedByUnicode13 holds the code points assigned by Unicode 13. A label
// is rejected for starting with a combining mark only when the mark is in
// this set: parsers with older mark tables (Node's new URL() among them)
// accept labels that start with a newer mark such as U+11F00.
var assignedByUnicode13 = rangetable.Assigned("13.0.0")

// failsDomainToASCII reports whether the WHATWG URL parser (used by browsers
// and the frontend's new URL()) rejects host in its "domain to ASCII" step,
// which url.Parse does not run. It checks each UTS46-mapped label for an
// xn-- label that does not decode to valid non-ASCII text, an unassigned or
// private-use code point or U+FFFD, a leading combining mark, and ZWNJ/ZWJ outside their
// CONTEXTJ contexts. An IPv6 literal with a zone ID fails too: url.Parse
// accepts "[fe80::1%25eth0]", browsers do not.
//
// Only these checks reject. A bare idna error is not enough: idna also flags
// labels a browser accepts.
func failsDomainToASCII(host string) bool {
	if strings.Contains(host, ":") {
		return strings.Contains(host, "%")
	}
	for raw := range strings.FieldsFuncSeq(host, isLabelSeparator) {
		// ToUnicode maps the label and decodes it if it is a valid xn--
		// label, returning that result even alongside an error.
		label, _ := whatwgIDNA.ToUnicode(raw)
		if isACELabel(raw) && !validDecodedACE(label) {
			// The WHATWG parser rejects it; Node's new URL() skips IDNA for
			// all-ASCII hosts and accepts it.
			return true
		}
		for i, r := range label {
			if r == unicode.ReplacementChar || unicode.In(r, unicode.Co, unicode.Cn) {
				return true
			}
			if i == 0 && unicode.Is(unicode.M, r) && unicode.Is(assignedByUnicode13, r) {
				return true
			}
		}
		// The leading "a" keeps idna's own leading-mark check out of this
		// one; a non-joining letter does not change any CONTEXTJ result.
		if strings.ContainsAny(label, "\u200c\u200d") {
			if _, err := joinerIDNA.ToUnicode("a" + label); err != nil {
				return true
			}
		}
	}
	return false
}

// isLabelSeparator reports the code points UTS46 maps to a label dot.
func isLabelSeparator(r rune) bool {
	return r == '.' || r == '\u3002' || r == '\uff0e' || r == '\uff61'
}

// isACELabel reports whether raw, once UTS46-mapped, starts with "xn--"
// (e.g. full-width "ｘｎ--"). The leading "a" stops ToUnicode decoding it.
func isACELabel(raw string) bool {
	mapped, _ := whatwgIDNA.ToUnicode("a" + raw)
	return strings.HasPrefix(mapped, "axn--")
}

// validDecodedACE reports whether label, the decoded form of an xn--
// label, is one UTS46 accepts: punycode that decoded (an undecodable label,
// such as "xn--zz" or "xn--é", stays "xn--..."), to non-ASCII text that
// UTS46 mapping leaves unchanged (no uppercase Ż from "xn--gha").
func validDecodedACE(label string) bool {
	if label == "" || isASCII(label) || strings.HasPrefix(label, "xn--") {
		return false
	}
	remapped, _ := whatwgIDNA.ToUnicode(label)
	return remapped == label
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// invalidNumericHost reports whether host is one the WHATWG URL parser
// (used by browsers and the frontend's new URL()) rejects because it ends in
// a number but is not a valid IPv4 address, e.g. "192.168.1.999",
// "1.2.3.4.5" or "chat.123". url.Parse accepts such hosts. IPv6 literals
// and non-numeric domains pass through unchanged.
func invalidNumericHost(host string) bool {
	if strings.Contains(host, ":") {
		return false
	}
	parts := strings.Split(host, ".")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if !endsInNumber(parts[len(parts)-1]) {
		return false
	}
	return !validIPv4(parts)
}

// endsInNumber mirrors the WHATWG "ends in a number" checker for the last
// host label: all decimal digits, or a 0x-prefixed hex number.
func endsInNumber(last string) bool {
	if last == "" {
		return false
	}
	if allDigits(last) {
		return true
	}
	_, ok := ipv4Number(last)
	return ok
}

// validIPv4 mirrors the WHATWG IPv4 parser over host labels with any
// trailing empty label already removed.
func validIPv4(parts []string) bool {
	if len(parts) > 4 {
		return false
	}
	nums := make([]uint64, len(parts))
	for i, p := range parts {
		n, ok := ipv4Number(p)
		if !ok {
			return false
		}
		if i < len(parts)-1 && n > 255 {
			return false
		}
		nums[i] = n
	}
	return nums[len(nums)-1] < 1<<(8*(5-len(parts)))
}

// ipv4Number parses one IPv4 label the WHATWG way: 0x/0X prefix is hex, a
// leading 0 is octal, otherwise decimal. A bare prefix ("0x") is zero.
func ipv4Number(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	base := 10
	switch {
	case len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X"):
		base, s = 16, s[2:]
	case len(s) >= 2 && s[0] == '0':
		base, s = 8, s[1:]
	}
	if s == "" {
		return 0, true
	}
	for _, r := range s {
		if d := digitValue(r); d < 0 || d >= base {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		// Every digit is valid, so the only error is overflow: still a
		// number, and one that fails the range check.
		return 1 << 63, true
	}
	return n, true
}

// digitValue returns the value of an ASCII hex digit, or -1.
func digitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return -1
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
