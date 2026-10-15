package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// APIError is every error body the API sends: RFC 9457 fields plus a stable,
// machine-readable code. Clients translate the code (the site will speak
// three languages); Detail is an Uzbek fallback for clients that do not.
type APIError struct {
	Status int              `json:"status" doc:"HTTP status code"`
	Code   string           `json:"code" doc:"Stable error code to translate, e.g. listing_not_found"`
	Title  string           `json:"title,omitempty" doc:"HTTP status text"`
	Detail string           `json:"detail,omitempty" doc:"Uzbek explanation, for clients without translations"`
	Errors []*APIFieldError `json:"errors,omitempty" doc:"One entry per rejected input"`
}

// APIFieldError is one rejected input: where, which rule and the rule's limits.
type APIFieldError struct {
	Location string         `json:"location,omitempty" doc:"Where the error occurred, e.g. body.price_som"`
	Code     string         `json:"code" doc:"Stable field error code, e.g. too_long"`
	Params   map[string]any `json:"params,omitempty" doc:"Values for the message, e.g. {\"max\": 60}"`
	Message  string         `json:"message,omitempty" doc:"Uzbek explanation"`
	Value    any            `json:"value,omitempty" doc:"The rejected value"`
}

func (p *APIError) Error() string  { return p.Code + ": " + p.Detail }
func (p *APIError) GetStatus() int { return p.Status }

// ContentType marks error bodies as problem documents (RFC 9457).
func (p *APIError) ContentType(ct string) string {
	if ct == "application/json" {
		return "application/problem+json"
	}
	return ct
}

// apiError is how handlers fail: a status, a code from the list below and the Uzbek text.
func apiError(status int, code, detail string, fields ...*APIFieldError) *APIError {
	return &APIError{Status: status, Code: code, Title: http.StatusText(status), Detail: detail, Errors: fields}
}

// fieldError rejects one body field.
func fieldError(field, code, message string, params map[string]any) *APIFieldError {
	return &APIFieldError{Location: "body." + field, Code: code, Message: message, Params: params}
}

// invalidInput is the 422 wrapper around rejected fields.
func invalidInput(fields ...*APIFieldError) *APIError {
	return apiError(http.StatusUnprocessableEntity, "validation_failed", "Ma'lumotlarni tekshiring.", fields...)
}

// Codes for errors Huma and the router raise themselves (by status).
var statusCodes = map[int][2]string{
	http.StatusBadRequest:            {"bad_request", "So'rov noto'g'ri tuzilgan."},
	http.StatusUnauthorized:          {"unauthorized", "Avval Telegram orqali kiring."},
	http.StatusForbidden:             {"forbidden", "Bu amal uchun ruxsat yo'q."},
	http.StatusNotFound:              {"not_found", "Topilmadi."},
	http.StatusMethodNotAllowed:      {"method_not_allowed", "Bu amal qo'llab-quvvatlanmaydi."},
	http.StatusRequestEntityTooLarge: {"body_too_large", "Yuborilgan ma'lumot juda katta."},
	http.StatusUnsupportedMediaType:  {"unsupported_media_type", "Bu formatdagi ma'lumot qabul qilinmaydi."},
	http.StatusUnprocessableEntity:   {"validation_failed", "Ma'lumotlarni tekshiring."},
	http.StatusTooManyRequests:       {"rate_limited", "Juda ko'p so'rov. Bir daqiqadan keyin qayta urinib ko'ring."},
	http.StatusInternalServerError:   {"internal", "Serverda xatolik yuz berdi. Birozdan keyin qayta urinib ko'ring."},
}

func errorForStatus(status int) *APIError {
	c, ok := statusCodes[status]
	if !ok {
		if status >= 500 {
			c = statusCodes[http.StatusInternalServerError]
		} else {
			c = [2]string{"error", "So'rovni bajarib bo'lmadi."}
		}
	}
	return apiError(status, c[0], c[1])
}

// Huma's built-in validation messages (huma/validation/messages.go), turned
// into field codes with their limits as params.
var validationRules = []struct {
	re      *regexp.Regexp
	code    string
	message func(m []string) string
	params  func(m []string) map[string]any
}{
	{regexp.MustCompile(`^expected required property (\S+) to be present`), "required",
		func([]string) string { return "To'ldirilishi shart." }, nil},
	{regexp.MustCompile(`^expected length <= (\d+)`), "too_long",
		func(m []string) string { return "Ko'pi bilan " + m[1] + " belgi bo'lsin." }, limit("max")},
	{regexp.MustCompile(`^expected length >= (\d+)`), "too_short",
		func(m []string) string { return "Kamida " + m[1] + " belgi bo'lsin." }, limit("min")},
	{regexp.MustCompile(`^expected number >=? (\S+)`), "too_small",
		func(m []string) string { return m[1] + " dan kichik bo'lmasin." }, limit("min")},
	{regexp.MustCompile(`^expected number <=? (\S+)`), "too_large",
		func(m []string) string { return m[1] + " dan katta bo'lmasin." }, limit("max")},
	{regexp.MustCompile(`^expected array length <= (\d+)`), "too_many",
		func(m []string) string { return "Ko'pi bilan " + m[1] + " ta bo'lsin." }, limit("max")},
	{regexp.MustCompile(`^expected value to be one of`), "invalid_choice",
		func([]string) string { return "Ro'yxatdagi qiymatlardan birini tanlang." }, nil},
	{regexp.MustCompile(`^expected (number|integer|string|boolean|array|object)$`), "invalid_type",
		func([]string) string { return "Qiymat turi noto'g'ri." }, nil},
	{regexp.MustCompile(`^unexpected property`), "unknown_field",
		func([]string) string { return "Noma'lum maydon." }, nil},
}

func limit(name string) func(m []string) map[string]any {
	return func(m []string) map[string]any {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return map[string]any{name: n}
		}
		return map[string]any{name: m[1]}
	}
}

// translateDetail converts one of Huma's English validation details.
func translateDetail(d *huma.ErrorDetail) *APIFieldError {
	fp := &APIFieldError{Location: d.Location, Code: "invalid", Message: "Qiymat noto'g'ri.", Value: d.Value}
	for _, r := range validationRules {
		m := r.re.FindStringSubmatch(d.Message)
		if m == nil {
			continue
		}
		fp.Code, fp.Message = r.code, r.message(m)
		if r.params != nil {
			fp.Params = r.params(m)
		}
		// "required property x" is reported on the parent; point at the field itself.
		if r.code == "required" && !strings.HasSuffix(fp.Location, "."+m[1]) {
			fp.Location = strings.TrimSuffix(fp.Location, ".") + "." + m[1]
		}
		break
	}
	return fp
}

// newError replaces huma.NewError, so errors Huma raises itself (validation,
// malformed bodies) get codes too. The first call builds the OpenAPI schema.
func newError(status int, _ string, errs ...error) huma.StatusError {
	p := errorForStatus(status)
	for _, err := range errs {
		switch e := err.(type) {
		case nil:
		case *APIFieldError:
			p.Errors = append(p.Errors, e)
		case huma.ErrorDetailer:
			p.Errors = append(p.Errors, translateDetail(e.ErrorDetail()))
		default:
			p.Errors = append(p.Errors, &APIFieldError{Code: "invalid", Message: "Qiymat noto'g'ri."})
		}
	}
	return p
}

func (f *APIFieldError) Error() string { return f.Location + ": " + f.Code }

// writeError answers outside Huma (router 404/405, rate limit) in the same shape.
func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorForStatus(status))
}
