package tradingview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// searchURL is the public, undocumented symbol-search REST endpoint
// used by the TradingView website's autocomplete.
const searchURL = "https://symbol-search.tradingview.com/symbol_search/"

type rawSymbolMatch struct {
	Symbol      string   `json:"symbol"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Exchange    string   `json:"exchange"`
	Prefix      string   `json:"prefix,omitempty"`
	Currency    string   `json:"currency_code"`
	SourceID    string   `json:"source_id"`
	TypeSpecs   []string `json:"typespecs"`
}

// SearchSymbols hits the public TradingView symbol-search endpoint and
// returns deduplicated, em-tag-stripped matches. Caller may pass an
// empty SearchOptions for a broad query.
func (c *Client) SearchSymbols(ctx context.Context, query string, opts SearchOptions) ([]SymbolMatch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, newErr(CodeInvalidBars, "search query is empty", nil)
	}
	v := url.Values{}
	v.Set("text", query)
	v.Set("hl", "1")
	if lang := opts.Lang; lang != "" {
		v.Set("lang", lang)
	} else {
		v.Set("lang", "en")
	}
	if opts.Type != "" {
		v.Set("type", opts.Type)
	}
	if opts.Exchange != "" {
		v.Set("exchange", strings.ToUpper(opts.Exchange))
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.SearchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, searchURL+"?"+v.Encode(), nil)
	if err != nil {
		return nil, newErr(CodeUpstreamHTTP, "build request", err)
	}
	req.Header.Set("Origin", "https://www.tradingview.com")
	req.Header.Set("Referer", "https://www.tradingview.com/")
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, newErr(CodeUpstreamHTTP, "search request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, newErr(CodeUpstreamHTTP, fmt.Sprintf("search HTTP %d: %s", resp.StatusCode, body), nil)
	}

	var raw []rawSymbolMatch
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, newErr(CodeUpstreamHTTP, "decode search response", err)
	}

	out := make([]SymbolMatch, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		m := SymbolMatch{
			Symbol:      stripEm(r.Symbol),
			Description: stripEm(r.Description),
			Type:        r.Type,
			Exchange:    r.Exchange,
			Prefix:      r.Prefix,
			Currency:    r.Currency,
			SourceID:    r.SourceID,
			TypeSpecs:   r.TypeSpecs,
		}
		key := m.Full() + "|" + m.Type
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

// stripEm removes the <em>...</em> wrapping the search endpoint adds
// around matching substrings.
func stripEm(s string) string {
	s = strings.ReplaceAll(s, "<em>", "")
	s = strings.ReplaceAll(s, "</em>", "")
	return s
}
