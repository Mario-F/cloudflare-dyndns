package cloudflare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
)

const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

const maxRetryDelay = 5 * time.Second

// Client talks to the Cloudflare API v4. It is not safe for concurrent use.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	RetryDelay time.Duration
	token      string
	zones      []Zone
}

type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Record struct {
	ID       string `json:"id"`
	ZoneID   string `json:"-"`
	ZoneName string `json:"-"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Proxied  bool   `json:"proxied"`
}

// Ref is the stable "<zone_id>/<record_id>" reference of a record.
func (r Record) Ref() string {
	return r.ZoneID + "/" + r.ID
}

type APIError struct {
	Status int
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
}

func (e *APIError) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("cloudflare API error (HTTP %d)", e.Status)
	}
	msgs := make([]string, 0, len(e.Errors))
	for _, m := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%d: %s", m.Code, m.Message))
	}
	return fmt.Sprintf("cloudflare API error (HTTP %d): %s", e.Status, strings.Join(msgs, "; "))
}

type envelope struct {
	Success    bool            `json:"success"`
	Errors     json.RawMessage `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo *struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func New(token string) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTP:       &http.Client{Timeout: 15 * time.Second},
		RetryDelay: time.Second,
		token:      token,
	}
}

// do performs one API call and retries once on 429 and 5xx.
func (c *Client) do(method, path string, query url.Values, body any, out any) (*envelope, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	u := strings.TrimRight(c.BaseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var resp *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(method, u, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		logger.Debugf("Cloudflare request: %s %s", method, path)
		resp, err = c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if attempt == 0 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			delay := c.retryDelay(resp.Header.Get("Retry-After"))
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			logger.Debugf("Cloudflare answered HTTP %d, retrying in %s", resp.StatusCode, delay)
			time.Sleep(delay)
			continue
		}
		break
	}
	defer func() { _ = resp.Body.Close() }()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("cloudflare API: unexpected response (HTTP %d): %w", resp.StatusCode, err)
	}
	if !env.Success {
		apiErr := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(env.Errors, &apiErr.Errors)
		return nil, apiErr
	}
	if out != nil {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return nil, fmt.Errorf("cloudflare API: decode result: %w", err)
		}
	}
	return &env, nil
}

func (c *Client) retryDelay(retryAfter string) time.Duration {
	delay := c.RetryDelay
	if s, err := strconv.Atoi(retryAfter); err == nil && s >= 0 {
		delay = time.Duration(s) * time.Second
	}
	return min(delay, maxRetryDelay)
}

func getAll[T any](c *Client, path string, query url.Values) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		var items []T
		env, err := c.do(http.MethodGet, path, query, nil, &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if env.ResultInfo == nil || page >= env.ResultInfo.TotalPages {
			return all, nil
		}
	}
}

// Verify checks that the token works and can read zones.
func (c *Client) Verify() error {
	if _, err := c.do(http.MethodGet, "/zones", url.Values{"per_page": {"1"}}, nil, nil); err != nil {
		return fmt.Errorf("token check failed (needs Zone:Read and DNS:Edit): %w", err)
	}
	return nil
}

func (c *Client) Zones() ([]Zone, error) {
	if c.zones != nil {
		return c.zones, nil
	}
	zones, err := getAll[Zone](c, "/zones", url.Values{"per_page": {"50"}})
	if err != nil {
		return nil, err
	}
	c.zones = zones
	return zones, nil
}

// Records lists the records of a zone with the given type, optionally filtered by exact name.
func (c *Client) Records(zone Zone, recordType, name string) ([]Record, error) {
	q := url.Values{"type": {recordType}, "per_page": {"100"}}
	if name != "" {
		q.Set("name", name)
	}
	records, err := getAll[Record](c, "/zones/"+url.PathEscape(zone.ID)+"/dns_records", q)
	if err != nil {
		return nil, err
	}
	for i := range records {
		records[i].ZoneID = zone.ID
		records[i].ZoneName = zone.Name
	}
	return records, nil
}

// FindZone returns the zone with the longest name matching the FQDN.
func FindZone(zones []Zone, fqdn string) (Zone, bool) {
	var best Zone
	found := false
	for _, z := range zones {
		name := strings.ToLower(strings.TrimSuffix(z.Name, "."))
		if (fqdn == name || strings.HasSuffix(fqdn, "."+name)) && len(name) > len(best.Name) {
			best = Zone{ID: z.ID, Name: name}
			found = true
		}
	}
	return best, found
}

// Record resolves a reference, either a FQDN or "<zone_id>/<record_id>", to exactly one record.
func (c *Client) Record(ref, recordType string) (Record, error) {
	if zoneID, recordID, ok := strings.Cut(ref, "/"); ok {
		if zoneID == "" || recordID == "" || strings.Contains(recordID, "/") {
			return Record{}, fmt.Errorf("invalid record reference %q, expected FQDN or <zone_id>/<record_id>", ref)
		}
		var r Record
		if _, err := c.do(http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(recordID), nil, nil, &r); err != nil {
			return Record{}, err
		}
		r.ZoneID = zoneID
		if r.Type != recordType {
			return Record{}, fmt.Errorf("record %s (%s) is of type %s, expected %s", ref, r.Name, r.Type, recordType)
		}
		return r, nil
	}

	fqdn := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(ref), "."))
	if fqdn == "" {
		return Record{}, errors.New("empty record reference")
	}
	zones, err := c.Zones()
	if err != nil {
		return Record{}, err
	}
	zone, ok := FindZone(zones, fqdn)
	if !ok {
		return Record{}, fmt.Errorf("no zone found for %s among the %d zones the token can access", fqdn, len(zones))
	}
	records, err := c.Records(zone, recordType, fqdn)
	if err != nil {
		return Record{}, err
	}
	switch len(records) {
	case 0:
		return Record{}, fmt.Errorf("no %s record %s found in zone %s (records are never created)", recordType, fqdn, zone.Name)
	case 1:
		return records[0], nil
	default:
		refs := make([]string, len(records))
		for i, r := range records {
			refs[i] = r.Ref()
		}
		return Record{}, fmt.Errorf("%s record %s is ambiguous (%s), use <zone_id>/<record_id>", recordType, fqdn, strings.Join(refs, ", "))
	}
}

// UpdateContent sets only the content of a record; proxied, ttl and comment stay as they are.
func (c *Client) UpdateContent(zoneID, recordID, content string) (Record, error) {
	var r Record
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records/" + url.PathEscape(recordID)
	if _, err := c.do(http.MethodPatch, path, nil, map[string]string{"content": content}, &r); err != nil {
		return Record{}, err
	}
	r.ZoneID = zoneID
	return r, nil
}
