// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
)

// pageSeqTransport replays responders in order, one per request. It lets a test
// make page 1 succeed and page 2 fail, which is the shape this file is about.
func pageSeqTransport(responders ...func() (*http.Response, error)) http.RoundTripper {
	i := 0
	return roundTripFunc(func(*http.Request) (*http.Response, error) {
		if i >= len(responders) {
			return nil, errors.New("unexpected extra request")
		}
		r := responders[i]
		i++
		return r()
	})
}

// firstPageHasMore is a successful page 1 that advertises a next page, so the
// loop is guaranteed to attempt page 2.
func firstPageHasMore() (*http.Response, error) {
	return jsonResponse(map[string]interface{}{
		"code": 0, "msg": "ok",
		"data": map[string]interface{}{
			"items":      []interface{}{map[string]interface{}{"id": "first"}},
			"has_more":   true,
			"page_token": "tok-2",
		},
	}), nil
}

// errSimulatedTransport is a stable sentinel so the tests can assert the
// transport error is carried through the pagination loop rather than merely
// classified as a network failure — the loop must not swallow the cause.
var errSimulatedTransport = errors.New("simulated transport failure")

func transportFailure() (*http.Response, error) {
	return nil, errSimulatedTransport
}

func TestPaginateAll_LaterPageTransportErrorPropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(firstPageHasMore, transportFailure))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want transport error from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryNetwork {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryNetwork)
	}
	if !errors.Is(err, errSimulatedTransport) {
		t.Errorf("errors.Is(err, errSimulatedTransport) = false; cause was not preserved; err = %v", err)
	}
}

func TestStreamPages_LaterPageTransportErrorPropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(firstPageHasMore, transportFailure))

	_, _, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, func([]interface{}) error { return nil },
		PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("StreamPages() error = nil, want transport error from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryNetwork {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryNetwork)
	}
	if !errors.Is(err, errSimulatedTransport) {
		t.Errorf("errors.Is(err, errSimulatedTransport) = false; cause was not preserved; err = %v", err)
	}
}

// businessFailure builds a page that failed with a non-zero business code.
// Such a page carries no data object, which is why the merged view used to
// report has_more=false and hide the failure.
func businessFailure(code int, msg string) func() (*http.Response, error) {
	return func() (*http.Response, error) {
		return jsonResponse(map[string]interface{}{"code": code, "msg": msg}), nil
	}
}

// Unknown codes fall through errclass's codemeta table into CategoryAPI.
func TestPaginateAll_LaterPageUnknownBusinessCodePropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, businessFailure(999999, "fixture unknown error")))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want business error from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryAPI {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryAPI)
	}

	p, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("errs.ProblemOf(err) = _, false; want a typed problem; err = %T: %v", err, err)
	}
	if p.Subtype != errs.SubtypeUnknown {
		t.Errorf("subtype = %q, want %q", p.Subtype, errs.SubtypeUnknown)
	}
}

// 230027 is in the codemeta table, so it classifies as authorization.
//
// The point of this case is the comparison in its name, so it runs both pages
// rather than asserting a hardcoded pair and calling that "matches page 1".
// Page 1 reaches its classification through the command layer: the loop hands
// back (failing page, nil) and CheckResponse turns it into the typed error.
// A later page is classified inside the loop. Both must land on the same
// category and subtype, or --page-all would report the same API failure
// differently depending on which page it arrived on.
func TestPaginateAll_LaterPageKnownBusinessCodeMatchesPageOneClassification(t *testing.T) {
	const code, msg = 230027, "user not authorized"
	opts := PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"}
	req := RawApiRequest{Method: "GET", URL: "/open-apis/test", As: "bot"}

	laterAC, _ := newTestAPIClient(t, pageSeqTransport(firstPageHasMore, businessFailure(code, msg)))
	_, laterErr := laterAC.PaginateAll(context.Background(), req, opts)
	if laterErr == nil {
		t.Fatal("PaginateAll() error = nil, want business error from page 2")
	}

	firstAC, _ := newTestAPIClient(t, pageSeqTransport(businessFailure(code, msg)))
	firstResult, err := firstAC.PaginateAll(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("PaginateAll() on a failing page 1 = %v, want nil so the command layer can dump the raw response", err)
	}
	firstErr := firstAC.CheckResponse(firstResult, opts.Identity)
	if firstErr == nil {
		t.Fatal("CheckResponse() on a failing page 1 = nil, want the typed error the command layer reports")
	}

	if got, want := errs.CategoryOf(laterErr), errs.CategoryOf(firstErr); got != want {
		t.Errorf("category: later page = %q, page 1 = %q; want identical", got, want)
	}
	if got := errs.CategoryOf(laterErr); got != errs.CategoryAuthorization {
		t.Errorf("errs.CategoryOf(laterErr) = %q, want %q", got, errs.CategoryAuthorization)
	}

	laterP, ok := errs.ProblemOf(laterErr)
	if !ok {
		t.Fatalf("errs.ProblemOf(laterErr) = _, false; want a typed problem; err = %T: %v", laterErr, laterErr)
	}
	firstP, ok := errs.ProblemOf(firstErr)
	if !ok {
		t.Fatalf("errs.ProblemOf(firstErr) = _, false; want a typed problem; err = %T: %v", firstErr, firstErr)
	}
	if laterP.Subtype != firstP.Subtype {
		t.Errorf("subtype: later page = %q, page 1 = %q; want identical", laterP.Subtype, firstP.Subtype)
	}
	if laterP.Subtype != errs.SubtypeUserUnauthorized {
		t.Errorf("subtype = %q, want %q", laterP.Subtype, errs.SubtypeUserUnauthorized)
	}
}

// The streaming path already surfaced non-zero codes before this change (the
// failing page was appended and returned as the last result, where the command
// layer's CheckResponse caught it). This case locks that behaviour against
// regression now that the error is built inside the loop instead.
//
// The "[pagination] streamed N pages" summary has its own case below; this one
// stays scoped to the error the caller receives.
func TestStreamPages_LaterPageKnownBusinessCodePropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, businessFailure(230027, "user not authorized")))

	_, _, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET",
		URL:    "/open-apis/test",
		As:     "bot",
	}, func([]interface{}) error { return nil },
		PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("StreamPages() error = nil, want business error from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryAuthorization {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryAuthorization)
	}
}

// statusResponse builds a page with an explicit HTTP status and a raw body,
// which jsonResponse cannot express (it hardcodes 200 and marshals a value).
func statusResponse(status int, body string) func() (*http.Response, error) {
	return func() (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
}

// A gateway 5xx carries no business code, so the code!=0 branch never fires and
// the page used to be appended as a success. HandleResponse already classifies
// this shape by status; the pagination path must not be the one place that
// swallows it.
func TestPaginateAll_LaterPageHTTPErrorWithoutBusinessCodePropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, statusResponse(502, `{"msg":"Bad Gateway"}`)))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want HTTP 502 from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryNetwork {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryNetwork)
	}
}

// A 200 whose body is JSON null leaves result == nil, so the map assertion that
// guards the code check fails and the loop breaks with pageToken empty.
func TestPaginateAll_LaterPageNullBodyPropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, statusResponse(200, `null`)))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want failure for an uninterpretable page 2")
	}
}

// Page 1 promised more data; a later page with no code field cannot be read as
// a valid continuation, so reporting the run as complete is the same lie #2477
// is about.
func TestPaginateAll_LaterPageObjectWithoutCodePropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, statusResponse(200, `{"data":{"items":[]}}`)))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want failure for a page 2 carrying no code")
	}
}

// Ordering guard: a response that carries BOTH a 4xx status and a non-zero
// business code must still classify by the code, the way HandleResponse does
// (check() runs before the status branch). Classifying by status here would
// downgrade 230027 from authorization to a generic HTTP error and, on page 1,
// would skip the raw-response dump the command layer owns.
func TestPaginateAll_LaterPageBusinessCodeWinsOverHTTPStatus(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, statusResponse(403, `{"code":230027,"msg":"user not authorized"}`)))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want business error from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryAuthorization {
		t.Errorf("errs.CategoryOf(err) = %q, want %q (status must not preempt the business code)",
			got, errs.CategoryAuthorization)
	}
}

// Without --page-all the same 502 is an error via HandleResponse. The paginated
// path must agree, or one command reports two different exit codes for one
// response depending on a flag.
func TestPaginateAll_FirstPageHTTPErrorPropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(statusResponse(502, `{"msg":"Bad Gateway"}`)))

	_, err := ac.PaginateAll(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("PaginateAll() error = nil, want HTTP 502 from page 1")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryNetwork {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryNetwork)
	}
}

func TestStreamPages_LaterPageHTTPErrorWithoutBusinessCodePropagates(t *testing.T) {
	ac, _ := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, statusResponse(502, `{"msg":"Bad Gateway"}`)))

	_, _, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, func([]interface{}) error { return nil },
		PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("StreamPages() error = nil, want HTTP 502 from page 2")
	}
	if got := errs.CategoryOf(err); got != errs.CategoryNetwork {
		t.Errorf("errs.CategoryOf(err) = %q, want %q", got, errs.CategoryNetwork)
	}
}

// The [page N] progress lines are the only place the loop treats page 1 and a
// later page differently on the failure path, and paginateLoop's comment names
// that guard specifically. It has to be asserted here, on the client's own
// ErrOut: the cmd-layer harnesses point ac.ErrOut at io.Discard, so the errOut
// assertions there check the command's output and can never fail on this.
func TestPaginateAll_StoppingLineIsLaterPagesOnly(t *testing.T) {
	req := RawApiRequest{Method: "GET", URL: "/open-apis/test", As: "bot"}
	opts := PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"}

	t.Run("later page reports why it stopped", func(t *testing.T) {
		ac, errBuf := newTestAPIClient(t, pageSeqTransport(firstPageHasMore, transportFailure))
		if _, err := ac.PaginateAll(context.Background(), req, opts); err == nil {
			t.Fatal("PaginateAll() error = nil, want transport error from page 2")
		}
		if want := "[page 2] error, stopping pagination"; !strings.Contains(errBuf.String(), want) {
			t.Errorf("ErrOut = %q, want it to contain %q", errBuf.String(), want)
		}
	})

	t.Run("page 1 does not, having stopped nothing", func(t *testing.T) {
		ac, errBuf := newTestAPIClient(t, pageSeqTransport(transportFailure))
		if _, err := ac.PaginateAll(context.Background(), req, opts); err == nil {
			t.Fatal("PaginateAll() error = nil, want transport error from page 1")
		}
		if strings.Contains(errBuf.String(), "stopping pagination") {
			t.Errorf("ErrOut = %q, want no \"stopping pagination\" line for a page-1 failure", errBuf.String())
		}
	})
}

// Streaming formats leave the successful pages on stdout, so the run's extent
// is information the caller cannot recover from the exit code alone.
func TestStreamPages_FailureReportsHowMuchWasStreamed(t *testing.T) {
	ac, errBuf := newTestAPIClient(t, pageSeqTransport(
		firstPageHasMore, businessFailure(230027, "user not authorized")))

	_, _, err := ac.StreamPages(context.Background(), RawApiRequest{
		Method: "GET", URL: "/open-apis/test", As: "bot",
	}, func([]interface{}) error { return nil },
		PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: "bot"})

	if err == nil {
		t.Fatal("StreamPages() error = nil, want business error from page 2")
	}
	want := "[pagination] streamed 1 pages, 1 total items before the run failed"
	if !strings.Contains(errBuf.String(), want) {
		t.Errorf("ErrOut = %q, want it to contain %q", errBuf.String(), want)
	}
}
