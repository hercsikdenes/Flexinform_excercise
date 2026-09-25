package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"carservice/internal/model"
	"carservice/internal/repository"
)

type fakeRepository struct {
	listClientsFunction            func(context.Context, int, int) ([]model.Client, int64, error)
	findClientsByNameFunction      func(context.Context, string, int) ([]model.Client, error)
	findClientByPersonalIDFunction func(context.Context, string) (model.Client, error)
	listClientCarsFunction         func(context.Context, string) ([]repository.ClientCar, error)
	getEventHistoryFunction        func(context.Context, string, int) (repository.History, error)
}

func (fakeRepository *fakeRepository) ListClients(ctx context.Context, limit, offset int) ([]model.Client, int64, error) {
	return fakeRepository.listClientsFunction(ctx, limit, offset)
}
func (fakeRepository *fakeRepository) FindClientsByName(ctx context.Context, name string, limit int) ([]model.Client, error) {
	return fakeRepository.findClientsByNameFunction(ctx, name, limit)
}
func (fakeRepository *fakeRepository) FindClientByPersonalID(ctx context.Context, id string) (model.Client, error) {
	return fakeRepository.findClientByPersonalIDFunction(ctx, id)
}
func (fakeRepository *fakeRepository) ListClientCars(ctx context.Context, id string) ([]repository.ClientCar, error) {
	return fakeRepository.listClientCarsFunction(ctx, id)
}
func (fakeRepository *fakeRepository) GetEventHistory(ctx context.Context, clientID string, carID int) (repository.History, error) {
	return fakeRepository.getEventHistoryFunction(ctx, clientID, carID)
}

func baseFake() *fakeRepository {
	return &fakeRepository{
		listClientsFunction:            func(context.Context, int, int) ([]model.Client, int64, error) { return nil, 0, nil },
		findClientsByNameFunction:      func(context.Context, string, int) ([]model.Client, error) { return nil, nil },
		findClientByPersonalIDFunction: func(context.Context, string) (model.Client, error) { return model.Client{}, repository.ErrNotFound },
		listClientCarsFunction:         func(context.Context, string) ([]repository.ClientCar, error) { return nil, nil },
		getEventHistoryFunction:        func(context.Context, string, int) (repository.History, error) { return repository.History{}, nil },
	}
}

func TestListClientsPagination(testContext *testing.T) {
	fake := baseFake()
	fake.listClientsFunction = func(_ context.Context, limit, offset int) ([]model.Client, int64, error) {
		if limit != 25 || offset != 25 {
			testContext.Fatalf("unexpected pagination: limit=%d offset=%d", limit, offset)
		}
		return []model.Client{{ID: "2", Name: "Second", PersonalID: "B2"}}, 26, nil
	}
	clientService := NewClientService(fake)
	page, err := clientService.ListClients(context.Background(), 2, 25)
	if err != nil {
		testContext.Fatalf("ListClients returned error: %v", err)
	}
	if page.Page != 2 || page.PerPage != 25 || page.Total != 26 || len(page.Data) != 1 || page.Data[0].ID != "2" {
		testContext.Fatalf("unexpected page: %+v", page)
	}

	for _, input := range [][2]int{{0, 10}, {1, 0}, {1, MaxPerPage + 1}, {math.MaxInt, 2}} {
		if _, err := clientService.ListClients(context.Background(), input[0], input[1]); !errors.Is(err, ErrInvalidInput) {
			testContext.Errorf("expected invalid input for page=%d perPage=%d, got %v", input[0], input[1], err)
		}
	}
}

func TestSearchValidationAndResults(testContext *testing.T) {
	fake := baseFake()
	clientService := NewClientService(fake)
	name, personalID := "Luke", "ABC123"
	invalidID := "bad-id"

	tests := []struct {
		name  string
		input SearchInput
		want  error
	}{
		{name: "neither", input: SearchInput{}, want: ErrInvalidInput},
		{name: "both", input: SearchInput{Name: &name, PersonalID: &personalID}, want: ErrInvalidInput},
		{name: "invalid personal id", input: SearchInput{PersonalID: &invalidID}, want: ErrInvalidInput},
	}
	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			if _, err := clientService.SearchClient(context.Background(), test.input); !errors.Is(err, test.want) {
				testContext.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}

	if _, err := clientService.SearchClient(context.Background(), SearchInput{PersonalID: &personalID}); !errors.Is(err, ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		testContext.Fatalf("repository not-found error was not translated: %v", err)
	}

	fake.findClientByPersonalIDFunction = func(_ context.Context, value string) (model.Client, error) {
		if value != personalID {
			testContext.Fatalf("personal id was not passed exactly: %q", value)
		}
		return model.Client{ID: "1", Name: "Luke", PersonalID: personalID}, nil
	}
	client, err := clientService.SearchClient(context.Background(), SearchInput{PersonalID: &personalID})
	if err != nil || client.PersonalID != personalID {
		testContext.Fatalf("unexpected exact personal id result: %+v, %v", client, err)
	}

	fake.findClientsByNameFunction = func(_ context.Context, value string, limit int) ([]model.Client, error) {
		if value != name || limit != 2 {
			testContext.Fatalf("unexpected name search arguments: %q %d", value, limit)
		}
		return []model.Client{{ID: "1", Name: "Luke Reid", PersonalID: "1"}}, nil
	}
	client, err = clientService.SearchClient(context.Background(), SearchInput{Name: &name})
	if err != nil || client.Name != "Luke Reid" {
		testContext.Fatalf("unexpected partial-name result: %+v, %v", client, err)
	}

	fake.findClientsByNameFunction = func(context.Context, string, int) ([]model.Client, error) { return nil, nil }
	if _, err := clientService.SearchClient(context.Background(), SearchInput{Name: &name}); !errors.Is(err, ErrNotFound) {
		testContext.Fatalf("expected not found, got %v", err)
	}
	fake.findClientsByNameFunction = func(context.Context, string, int) ([]model.Client, error) {
		return []model.Client{{ID: "1"}, {ID: "2"}}, nil
	}
	if _, err := clientService.SearchClient(context.Background(), SearchInput{Name: &name}); !errors.Is(err, ErrAmbiguousSearch) {
		testContext.Fatalf("expected ambiguous search, got %v", err)
	}
}

func TestCarsAndHistoryBusinessRules(testContext *testing.T) {
	registered := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	eventName := "registered"
	logNumber := 1
	fake := baseFake()
	fake.listClientCarsFunction = func(context.Context, string) ([]repository.ClientCar, error) {
		return []repository.ClientCar{{
			CarID: 1, VehicleID: 99, Type: "SZGK", RegisteredAt: registered,
			LatestLogNumber: &logNumber, LatestEvent: &eventName,
		}}, nil
	}
	clientService := NewClientService(fake)
	cars, err := clientService.ListClientCars(context.Background(), "client")
	if err != nil || len(cars) != 1 || cars[0].LatestEvent == nil || !cars[0].LatestEvent.EventTime.Equal(registered) {
		testContext.Fatalf("missing latest-event timestamp was not replaced: %+v, %v", cars, err)
	}

	fake.listClientCarsFunction = func(context.Context, string) ([]repository.ClientCar, error) { return []repository.ClientCar{}, nil }
	cars, err = clientService.ListClientCars(context.Background(), "client")
	if err != nil || cars == nil || len(cars) != 0 {
		testContext.Fatalf("known client with no cars should return a non-nil empty list: %#v, %v", cars, err)
	}
	fake.listClientCarsFunction = func(context.Context, string) ([]repository.ClientCar, error) { return nil, repository.ErrNotFound }
	if _, err := clientService.ListClientCars(context.Background(), "missing"); !errors.Is(err, ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		testContext.Fatalf("repository not-found error was not translated: %v", err)
	}

	document := "123"
	fake = baseFake()
	fake.getEventHistoryFunction = func(context.Context, string, int) (repository.History, error) {
		return repository.History{
			ClientID: "client", CarID: 1, VehicleID: 99, RegisteredAt: registered,
			CarsCount: 2, ServicesCount: 2,
			Events: []repository.Event{
				{LogNumber: 1, Event: "registered", DocumentID: nil},
				{LogNumber: 2, Event: "service", EventTime: &registered, DocumentID: &document},
			},
		}, nil
	}
	clientService = NewClientService(fake)
	history, err := clientService.GetEventHistory(context.Background(), "client", 1)
	if err != nil || history.CarsCount != 2 || history.ServicesCount != 2 || len(history.Services) != 2 {
		testContext.Fatalf("unexpected history: %+v, %v", history, err)
	}
	if !history.Services[0].EventTime.Equal(registered) || history.Services[0].DocumentID != nil {
		testContext.Fatalf("history fallback/null normalization failed: %+v", history.Services[0])
	}

	fake.getEventHistoryFunction = func(context.Context, string, int) (repository.History, error) {
		return repository.History{}, repository.ErrNotFound
	}
	if _, err := clientService.GetEventHistory(context.Background(), "client", 1); !errors.Is(err, ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		testContext.Fatalf("repository not-found error was not translated: %v", err)
	}
}
