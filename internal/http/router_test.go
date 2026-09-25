package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"carservice/internal/service"
	"github.com/gin-gonic/gin"
)

type stubService struct {
	listPage        service.ClientPage
	listErr         error
	page            int
	perPage         int
	search          service.Client
	searchErr       error
	cars            []service.ClientCar
	carsErr         error
	history         service.EventHistory
	historyErr      error
	searchInput     service.SearchInput
	carsClientID    string
	historyClientID string
	historyCarID    int
}

func (stub *stubService) ListClients(_ context.Context, page, perPage int) (service.ClientPage, error) {
	stub.page, stub.perPage = page, perPage
	return stub.listPage, stub.listErr
}
func (stub *stubService) SearchClient(_ context.Context, input service.SearchInput) (service.Client, error) {
	stub.searchInput = input
	return stub.search, stub.searchErr
}
func (stub *stubService) ListClientCars(_ context.Context, clientID string) ([]service.ClientCar, error) {
	stub.carsClientID = clientID
	return stub.cars, stub.carsErr
}
func (stub *stubService) GetEventHistory(_ context.Context, clientID string, carID int) (service.EventHistory, error) {
	stub.historyClientID = clientID
	stub.historyCarID = carID
	return stub.history, stub.historyErr
}

type stubHealth struct {
	err          error
	pingFunction func(context.Context) error
}

func (health stubHealth) Ping(requestContext context.Context) error {
	if health.pingFunction != nil {
		return health.pingFunction(requestContext)
	}
	return health.err
}

func TestPaginationHTTPBehaviour(testContext *testing.T) {
	ginTestMode(testContext)
	stub := &stubService{listPage: service.ClientPage{Data: []service.Client{}, Page: 1, PerPage: 50}}
	router := NewRouter(stub, stubHealth{}, log.New(&bytes.Buffer{}, "", 0))

	response := performRequest(router, http.MethodGet, "/api/clients")
	if response.Code != http.StatusOK || stub.page != 1 || stub.perPage != 50 {
		testContext.Fatalf("default pagination failed: status=%d page=%d perPage=%d", response.Code, stub.page, stub.perPage)
	}
	response = performRequest(router, http.MethodGet, "/api/clients?page=2&per_page=25")
	if response.Code != http.StatusOK || stub.page != 2 || stub.perPage != 25 {
		testContext.Fatalf("explicit pagination failed: status=%d page=%d perPage=%d", response.Code, stub.page, stub.perPage)
	}
	for _, path := range []string{"/api/clients?page=0", "/api/clients?per_page=x"} {
		response = performRequest(router, http.MethodGet, path)
		if response.Code != http.StatusUnprocessableEntity {
			testContext.Errorf("%s returned %d", path, response.Code)
		}
	}
}

func TestSuccessfulClientRoutes(testContext *testing.T) {
	ginTestMode(testContext)
	registeredAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	documentID := "document-1"
	stub := &stubService{
		search: service.Client{ID: "client-1", Name: "Luke Reid", PersonalID: "ABC123"},
		cars: []service.ClientCar{{
			CarID: 1, VehicleID: 10, Type: "SZGK", RegisteredAt: registeredAt,
			LatestEvent: &service.Event{LogNumber: 2, Event: "service", EventTime: registeredAt, DocumentID: &documentID},
		}},
		history: service.EventHistory{
			ClientID: "client-1", CarID: 1, VehicleID: 10, CarsCount: 1, ServicesCount: 1,
			Services: []service.Event{{LogNumber: 2, Event: "service", EventTime: registeredAt, DocumentID: &documentID}},
		},
	}
	router := NewRouter(stub, stubHealth{}, log.New(&bytes.Buffer{}, "", 0))

	response := performRequest(router, http.MethodGet, "/api/clients/search?name=Luke")
	var client service.Client
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &client) != nil || client.ID != "client-1" {
		testContext.Fatalf("unexpected search response: %d %s", response.Code, response.Body.String())
	}
	if stub.searchInput.Name == nil || *stub.searchInput.Name != "Luke" || stub.searchInput.PersonalID != nil {
		testContext.Fatalf("unexpected search input: %+v", stub.searchInput)
	}

	response = performRequest(router, http.MethodGet, "/api/clients/client-1/cars")
	var cars []service.ClientCar
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &cars) != nil || len(cars) != 1 || cars[0].LatestEvent == nil {
		testContext.Fatalf("unexpected cars response: %d %s", response.Code, response.Body.String())
	}
	if stub.carsClientID != "client-1" {
		testContext.Fatalf("unexpected cars client id: %q", stub.carsClientID)
	}

	response = performRequest(router, http.MethodGet, "/api/clients/client-1/cars/1/services")
	var history service.EventHistory
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &history) != nil || len(history.Services) != 1 {
		testContext.Fatalf("unexpected history response: %d %s", response.Code, response.Body.String())
	}
	if stub.historyClientID != "client-1" || stub.historyCarID != 1 {
		testContext.Fatalf("unexpected history parameters: %q %d", stub.historyClientID, stub.historyCarID)
	}
}

func TestExpectedAndUnexpectedErrorResponses(testContext *testing.T) {
	ginTestMode(testContext)
	tests := []struct {
		name    string
		path    string
		status  int
		message string
		prepare func(*stubService)
	}{
		{
			name: "invalid pagination", path: "/api/clients", status: 422, message: "request input is invalid",
			prepare: func(stub *stubService) { stub.listErr = service.ErrInvalidInput },
		},
		{
			name: "ambiguous search", path: "/api/clients/search?name=Luke", status: 422, message: "name search returned multiple clients",
			prepare: func(stub *stubService) { stub.searchErr = service.ErrAmbiguousSearch },
		},
		{
			name: "missing client", path: "/api/clients/client/cars", status: 404, message: "requested resource was not found",
			prepare: func(stub *stubService) { stub.carsErr = service.ErrNotFound },
		},
		{
			name: "missing ownership", path: "/api/clients/client/cars/1/services", status: 404, message: "requested resource was not found",
			prepare: func(stub *stubService) { stub.historyErr = service.ErrNotFound },
		},
		{
			name: "unexpected", path: "/api/clients", status: 500, message: "an unexpected internal error occurred",
			prepare: func(stub *stubService) { stub.listErr = errors.New("secret SQL driver failure") },
		},
	}
	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			stub := &stubService{}
			test.prepare(stub)
			router := NewRouter(stub, stubHealth{}, log.New(&bytes.Buffer{}, "", 0))
			response := performRequest(router, http.MethodGet, test.path)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"message":"`+test.message+`"`) {
				testContext.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "SQL") || strings.Contains(response.Body.String(), "driver") {
				testContext.Fatalf("internal error leaked: %s", response.Body.String())
			}
		})
	}
}

func TestHealthAndUnknownRoute(testContext *testing.T) {
	ginTestMode(testContext)
	healthDeadlineObserved := false
	health := stubHealth{pingFunction: func(requestContext context.Context) error {
		deadline, exists := requestContext.Deadline()
		healthDeadlineObserved = exists && time.Until(deadline) > 0 && time.Until(deadline) <= 3*time.Second
		return nil
	}}
	router := NewRouter(&stubService{}, health, log.New(&bytes.Buffer{}, "", 0))
	response := performRequest(router, http.MethodGet, "/health")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"message":"ok"`) || !healthDeadlineObserved {
		testContext.Fatalf("healthy response: %d %s", response.Code, response.Body.String())
	}

	router = NewRouter(&stubService{}, stubHealth{err: errors.New("database down")}, log.New(&bytes.Buffer{}, "", 0))
	response = performRequest(router, http.MethodGet, "/health")
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "database down") {
		testContext.Fatalf("unhealthy response: %d %s", response.Code, response.Body.String())
	}

	response = performRequest(router, http.MethodGet, "/missing")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"message":"route not found"`) {
		testContext.Fatalf("unknown route response: %d %s", response.Code, response.Body.String())
	}
}

func TestMethodAndPanicResponses(testContext *testing.T) {
	ginTestMode(testContext)
	logBuffer := &bytes.Buffer{}
	router := NewRouter(&stubService{}, stubHealth{}, log.New(logBuffer, "", 0))
	router.GET("/panic", func(context *gin.Context) {
		panic("secret panic detail")
	})

	response := performRequest(router, http.MethodPost, "/health")
	if response.Code != http.StatusMethodNotAllowed || !strings.Contains(response.Body.String(), `"message":"method not allowed"`) {
		testContext.Fatalf("method response: %d %s", response.Code, response.Body.String())
	}

	response = performRequest(router, http.MethodGet, "/panic")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"message":"an unexpected internal error occurred"`) {
		testContext.Fatalf("panic response: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "secret panic detail") || !strings.Contains(logBuffer.String(), "secret panic detail") {
		testContext.Fatalf("panic detail handling failed: response=%s log=%s", response.Body.String(), logBuffer.String())
	}
}

func performRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func ginTestMode(testContext *testing.T) {
	testContext.Helper()
	testContext.Setenv("GIN_MODE", "test")
}
