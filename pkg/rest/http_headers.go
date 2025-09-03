package rest

import (
	"net/http"
	"strings"
)

// Prefer holds preferences from the Prefer header (RFC 7240).
type Prefer struct {
	Return string // "minimal", "representation", "headers-only"
	Count  string // "exact", "planned", "estimated"
}

// Headers holds all parsed HTTP headers relevant to REST actions.
type Headers struct {
	Prefer *Prefer
}

// parseHeaders parses all relevant headers from the HTTP request
func parseHeaders(r *http.Request) *Headers {
	h := &Headers{}
	h.Prefer = parsePrefer(r)
	return h
}

// parsePrefer parses the Prefer header according to RFC 7240.
// It returns nil if the header is not present.
func parsePrefer(r *http.Request) *Prefer {
	header := r.Header.Get("Prefer")
	if header == "" {
		return nil
	}

	p := &Prefer{
		Return: "minimal", // RFC 7240 default behavior
	}

	parseKeyValPairs(header, func(key, value string) {
		switch key {
		case "return":
			if isValidReturn(value) {
				p.Return = value
			}
		case "count":
			if isValidCount(value) {
				p.Count = value
			}
		}
	})

	return p
}

// parseKeyValPairs parses comma-separated preference directives.
// For each key=value pair found, it calls fn with the key and value.
func parseKeyValPairs(header string, fn func(key, value string)) {
	prefs := strings.SplitSeq(header, ",")
	for pref := range prefs {
		pref = strings.TrimSpace(pref)
		if key, value, found := strings.Cut(pref, "="); found {
			key = strings.TrimSpace(strings.ToLower(key))       // normalize case
			value = strings.Trim(strings.TrimSpace(value), `"`) // remove quotes
			fn(key, value)
		}
	}
}

// isValidReturn reports whether s is a valid return preference value.
func isValidReturn(s string) bool {
	s = strings.ToLower(s) // normalize case
	switch s {
	case "minimal", "representation", "headers-only":
		return true
	}
	return false
}

// isValidCount reports whether s is a valid count preference value.
func isValidCount(s string) bool {
	s = strings.ToLower(s) // normalize case
	switch s {
	case "exact", "planned", "estimated":
		return true
	}
	return false
}

// WantsRepresentation reports whether the client prefers full representation
// in the response body for mutation operations.
func (p *Prefer) WantsRepresentation() bool {
	return p != nil && p.Return == "representation"
}

// WantsHeadersOnly reports whether the client prefers only headers
// with no response body for mutation operations.
func (p *Prefer) WantsHeadersOnly() bool {
	return p != nil && p.Return == "headers-only"
}

// WantsCountExact reports whether the client wants an exact count in the response.
func (p *Prefer) WantsCountExact() bool {
	return p != nil && strings.ToLower(p.Count) == "exact"
}

// WantsCountEstimated reports whether the client wants an estimated count in the response.
func (p *Prefer) WantsCountEstimated() bool {
	return p != nil && strings.ToLower(p.Count) == "estimated"
}

// WantsCountPlanned reports whether the client wants a planned count in the response.
func (p *Prefer) WantsCountPlanned() bool {
	return p != nil && strings.ToLower(p.Count) == "planned"
}
