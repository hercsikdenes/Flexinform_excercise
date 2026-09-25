package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carservice/internal/model"

	"gorm.io/gorm"
)

type GORMClientRepository struct {
	db *gorm.DB
}

type historySummaryRow struct {
	OwnershipID   int
	VehicleID     int
	RegisteredAt  time.Time
	CarsCount     int64
	ServicesCount int64
}

type eventRow struct {
	LogNumber  int
	Event      string
	EventTime  sql.NullTime
	DocumentID sql.NullString
}

type clientCarRow struct {
	ClientID         string
	CarID            sql.NullInt64
	VehicleID        sql.NullInt64
	Type             sql.NullString
	RegisteredAt     sql.NullTime
	OwnBrand         sql.NullBool
	AccidentCount    sql.NullInt64
	LatestLogNumber  sql.NullInt64
	LatestEvent      sql.NullString
	LatestEventTime  sql.NullTime
	LatestDocumentID sql.NullString
}

func NewGORMClientRepository(db *gorm.DB) *GORMClientRepository {
	return &GORMClientRepository{db: db}
}

func (repository *GORMClientRepository) ListClients(ctx context.Context, limit, offset int) ([]model.Client, int64, error) {
	db := repository.db.WithContext(ctx).Model(&model.Client{}).Where("is_active = ?", true)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count active clients: %w", err)
	}
	clients := make([]model.Client, 0)
	if err := db.Order("id ASC").Limit(limit).Offset(offset).Find(&clients).Error; err != nil {
		return nil, 0, fmt.Errorf("list active clients: %w", err)
	}
	return clients, total, nil
}

func (repository *GORMClientRepository) FindClientsByName(ctx context.Context, name string, limit int) ([]model.Client, error) {
	clients := make([]model.Client, 0)
	pattern := "%" + name + "%"
	err := repository.db.WithContext(ctx).
		Where("is_active = ? AND LOWER(name) LIKE LOWER(?)", true, pattern).
		Order("id ASC").
		Limit(limit).
		Find(&clients).Error
	if err != nil {
		return nil, fmt.Errorf("search active clients by name: %w", err)
	}
	return clients, nil
}

func (repository *GORMClientRepository) FindClientByPersonalID(ctx context.Context, personalID string) (model.Client, error) {
	var client model.Client
	err := repository.db.WithContext(ctx).
		Where("is_active = ? AND personal_id = ?", true, personalID).
		First(&client).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Client{}, ErrNotFound
	}
	if err != nil {
		return model.Client{}, fmt.Errorf("find active client by personal id: %w", err)
	}
	return client, nil
}

func (repository *GORMClientRepository) ListClientCars(ctx context.Context, clientID string) ([]ClientCar, error) {
	const query = `
WITH active_cars AS (
	SELECT
		co.id AS ownership_id,
		co.client_id,
		co.client_car_number AS car_id,
		vehicle.id AS vehicle_id,
		vehicle.type,
		vehicle.registered_at,
		vehicle.own_brand,
		vehicle.accident_count
	FROM car_ownerships co
	JOIN vehicles vehicle ON vehicle.id = co.vehicle_id
	WHERE co.is_active = ?
	  AND vehicle.is_active = ?
)
SELECT
	client.id AS client_id,
	car.car_id,
	car.vehicle_id,
	car.type,
	car.registered_at,
	car.own_brand,
	car.accident_count,
    el.log_number AS latest_log_number,
    el.event AS latest_event,
    el.event_time AS latest_event_time,
    el.document_id AS latest_document_id
FROM clients client
LEFT JOIN active_cars car ON car.client_id = client.id
LEFT JOIN event_log el
	ON el.ownership_id = car.ownership_id
   AND el.log_number = (
       SELECT MAX(el2.log_number)
       FROM event_log el2
	   WHERE el2.ownership_id = car.ownership_id
   )
WHERE client.id = ?
  AND client.is_active = ?
ORDER BY car.car_id ASC`

	var rows []clientCarRow
	if err := repository.db.WithContext(ctx).Raw(query, true, true, clientID, true).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list client cars with latest events: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	result := make([]ClientCar, 0, len(rows))
	for _, row := range rows {
		if !row.CarID.Valid {
			continue
		}
		item := ClientCar{
			CarID: int(row.CarID.Int64), VehicleID: int(row.VehicleID.Int64), Type: row.Type.String,
			OwnBrand: row.OwnBrand.Bool, AccidentCount: int(row.AccidentCount.Int64),
		}
		if row.RegisteredAt.Valid {
			item.RegisteredAt = row.RegisteredAt.Time
		}
		if row.LatestLogNumber.Valid {
			value := int(row.LatestLogNumber.Int64)
			item.LatestLogNumber = &value
		}
		if row.LatestEvent.Valid {
			value := row.LatestEvent.String
			item.LatestEvent = &value
		}
		if row.LatestEventTime.Valid {
			value := row.LatestEventTime.Time
			item.LatestEventTime = &value
		}
		if row.LatestDocumentID.Valid {
			value := row.LatestDocumentID.String
			item.LatestDocumentID = &value
		}
		result = append(result, item)
	}
	return result, nil
}

func (repository *GORMClientRepository) GetEventHistory(ctx context.Context, clientID string, carID int) (History, error) {
	const summaryQuery = `
SELECT
    co.id AS ownership_id,
    vehicle.id AS vehicle_id,
    vehicle.registered_at,
    (
        SELECT COUNT(*)
        FROM car_ownerships co2
        JOIN vehicles v2 ON v2.id = co2.vehicle_id
        WHERE co2.client_id = co.client_id
          AND co2.is_active = ?
          AND v2.is_active = ?
    ) AS cars_count,
    (
        SELECT COUNT(*)
        FROM event_log count_events
        WHERE count_events.ownership_id = co.id
    ) AS services_count
FROM car_ownerships co
JOIN clients client ON client.id = co.client_id
JOIN vehicles vehicle ON vehicle.id = co.vehicle_id
WHERE co.client_id = ?
  AND co.client_car_number = ?
  AND client.is_active = ?
  AND co.is_active = ?
  AND vehicle.is_active = ?`

	var summary historySummaryRow

	result := repository.db.WithContext(ctx).Raw(
		summaryQuery, true, true, clientID, carID, true, true, true,
	).Scan(&summary)

	if result.Error != nil {
		return History{}, fmt.Errorf("resolve ownership history: %w", result.Error)
	}

	if result.RowsAffected == 0 || summary.OwnershipID == 0 {
		return History{}, ErrNotFound
	}

	const eventsQuery = `
SELECT log_number, event, event_time, document_id
FROM event_log
WHERE ownership_id = ?
ORDER BY log_number ASC`

	var rows []eventRow

	if err := repository.db.WithContext(ctx).Raw(eventsQuery, summary.OwnershipID).Scan(&rows).Error; err != nil {
		return History{}, fmt.Errorf("list ownership event history: %w", err)
	}

	events := make([]Event, 0, len(rows))

	for _, row := range rows {
		event := Event{LogNumber: row.LogNumber, Event: row.Event}
		if row.EventTime.Valid {
			value := row.EventTime.Time
			event.EventTime = &value
		}
		if row.DocumentID.Valid {
			value := row.DocumentID.String
			event.DocumentID = &value
		}
		events = append(events, event)
	}

	return History{
		ClientID: clientID, CarID: carID, VehicleID: summary.VehicleID,
		RegisteredAt: summary.RegisteredAt, CarsCount: summary.CarsCount,
		ServicesCount: summary.ServicesCount, Events: events,
	}, nil
}
