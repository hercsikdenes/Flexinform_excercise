package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"carservice/internal/model"
	"carservice/internal/repository"
)

const MaxPerPage = 100

var personalIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

type Client struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PersonalID string `json:"personal_id"`
}

type ClientPage struct {
	Data    []Client `json:"data"`
	Page    int      `json:"page"`
	PerPage int      `json:"per_page"`
	Total   int64    `json:"total"`
}

type SearchInput struct {
	Name       *string
	PersonalID *string
}

type Event struct {
	LogNumber  int       `json:"log_number"`
	Event      string    `json:"event"`
	EventTime  time.Time `json:"event_time"`
	DocumentID *string   `json:"document_id"`
}

type ClientCar struct {
	CarID         int       `json:"car_id"`
	VehicleID     int       `json:"vehicle_id"`
	Type          string    `json:"type"`
	RegisteredAt  time.Time `json:"registered_at"`
	OwnBrand      bool      `json:"own_brand"`
	AccidentCount int       `json:"accident_count"`
	LatestEvent   *Event    `json:"latest_event"`
}

type EventHistory struct {
	ClientID      string  `json:"client_id"`
	CarID         int     `json:"car_id"`
	VehicleID     int     `json:"vehicle_id"`
	CarsCount     int64   `json:"cars_count"`
	ServicesCount int64   `json:"services_count"`
	Services      []Event `json:"services"`
}

type ClientService struct {
	repository repository.ClientRepository
}

func NewClientService(clientRepository repository.ClientRepository) *ClientService {
	return &ClientService{repository: clientRepository}
}

func (clientService *ClientService) ListClients(ctx context.Context, page, perPage int) (ClientPage, error) {
	if page < 1 || perPage < 1 || perPage > MaxPerPage {
		return ClientPage{}, fmt.Errorf("pagination values: %w", ErrInvalidInput)
	}
	if page-1 > math.MaxInt/perPage {
		return ClientPage{}, fmt.Errorf("pagination offset: %w", ErrInvalidInput)
	}
	offset := (page - 1) * perPage
	clients, total, err := clientService.repository.ListClients(ctx, perPage, offset)
	if err != nil {
		return ClientPage{}, fmt.Errorf("list clients: %w", err)
	}
	result := make([]Client, 0, len(clients))
	for _, client := range clients {
		result = append(result, publicClient(client))
	}
	return ClientPage{Data: result, Page: page, PerPage: perPage, Total: total}, nil
}

func (clientService *ClientService) SearchClient(ctx context.Context, input SearchInput) (Client, error) {
	if (input.Name == nil) == (input.PersonalID == nil) {
		return Client{}, fmt.Errorf("exactly one search parameter is required: %w", ErrInvalidInput)
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Client{}, fmt.Errorf("name must not be empty: %w", ErrInvalidInput)
		}
		clients, err := clientService.repository.FindClientsByName(ctx, name, 2)
		if err != nil {
			return Client{}, fmt.Errorf("search client by name: %w", err)
		}
		switch len(clients) {
		case 0:
			return Client{}, ErrNotFound
		case 1:
			return publicClient(clients[0]), nil
		default:
			return Client{}, ErrAmbiguousSearch
		}
	}

	personalID := strings.TrimSpace(*input.PersonalID)
	if !personalIDPattern.MatchString(personalID) {
		return Client{}, fmt.Errorf("personal_id must contain only letters and numbers: %w", ErrInvalidInput)
	}
	client, err := clientService.repository.FindClientByPersonalID(ctx, personalID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Client{}, fmt.Errorf("search client by personal id: %w", ErrNotFound)
		}
		return Client{}, fmt.Errorf("search client by personal id: %w", err)
	}
	return publicClient(client), nil
}

func (clientService *ClientService) ListClientCars(ctx context.Context, clientID string) ([]ClientCar, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fmt.Errorf("client id must not be empty: %w", ErrInvalidInput)
	}

	records, err := clientService.repository.ListClientCars(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("list client cars: %w", ErrNotFound)
		}
		return nil, fmt.Errorf("list client cars: %w", err)
	}
	cars := make([]ClientCar, 0, len(records))
	for _, record := range records {
		car := ClientCar{
			CarID: record.CarID, VehicleID: record.VehicleID, Type: record.Type,
			RegisteredAt: record.RegisteredAt, OwnBrand: record.OwnBrand,
			AccidentCount: record.AccidentCount,
		}
		if record.LatestLogNumber != nil && record.LatestEvent != nil {
			eventTime := record.RegisteredAt
			if record.LatestEventTime != nil {
				eventTime = *record.LatestEventTime
			}
			car.LatestEvent = &Event{
				LogNumber: *record.LatestLogNumber, Event: *record.LatestEvent,
				EventTime: eventTime, DocumentID: record.LatestDocumentID,
			}
		}
		cars = append(cars, car)
	}
	return cars, nil
}

func (clientService *ClientService) GetEventHistory(ctx context.Context, clientID string, carID int) (EventHistory, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" || carID < 1 {
		return EventHistory{}, fmt.Errorf("client id and positive car id are required: %w", ErrInvalidInput)
	}
	history, err := clientService.repository.GetEventHistory(ctx, clientID, carID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return EventHistory{}, fmt.Errorf("get event history: %w", ErrNotFound)
		}
		return EventHistory{}, fmt.Errorf("get event history: %w", err)
	}
	events := make([]Event, 0, len(history.Events))
	for _, record := range history.Events {
		eventTime := history.RegisteredAt
		if record.EventTime != nil {
			eventTime = *record.EventTime
		}
		events = append(events, Event{
			LogNumber: record.LogNumber, Event: record.Event,
			EventTime: eventTime, DocumentID: record.DocumentID,
		})
	}
	return EventHistory{
		ClientID: history.ClientID, CarID: history.CarID, VehicleID: history.VehicleID,
		CarsCount: history.CarsCount, ServicesCount: history.ServicesCount, Services: events,
	}, nil
}

func publicClient(client model.Client) Client {
	return Client{ID: client.ID, Name: client.Name, PersonalID: client.PersonalID}
}
