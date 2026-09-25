package repository

import (
	"context"
	"errors"
	"time"

	"carservice/internal/model"
)

var ErrNotFound = errors.New("repository resource not found")

type ClientCar struct {
	CarID            int
	VehicleID        int
	Type             string
	RegisteredAt     time.Time
	OwnBrand         bool
	AccidentCount    int
	LatestLogNumber  *int
	LatestEvent      *string
	LatestEventTime  *time.Time
	LatestDocumentID *string
}

type Event struct {
	LogNumber  int
	Event      string
	EventTime  *time.Time
	DocumentID *string
}

type History struct {
	ClientID      string
	CarID         int
	VehicleID     int
	RegisteredAt  time.Time
	CarsCount     int64
	ServicesCount int64
	Events        []Event
}

type ClientRepository interface {
	ListClients(ctx context.Context, limit, offset int) ([]model.Client, int64, error)
	FindClientsByName(ctx context.Context, name string, limit int) ([]model.Client, error)
	FindClientByPersonalID(ctx context.Context, personalID string) (model.Client, error)
	ListClientCars(ctx context.Context, clientID string) ([]ClientCar, error)
	GetEventHistory(ctx context.Context, clientID string, carID int) (History, error)
}
