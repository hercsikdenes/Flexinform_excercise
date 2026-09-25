package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"carservice/internal/model"

	"gorm.io/gorm"
)

const (
	batchSize      = 500
	sourceTimeForm = "2006-01-02 15:04:05"
)

type Seeder struct {
	db      *gorm.DB
	dataDir string
}

func New(db *gorm.DB, dataDir string) *Seeder {
	return &Seeder{db: db, dataDir: dataDir}
}

func (seeder *Seeder) Ensure(ctx context.Context) error {
	return seeder.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := inspectState(tx)

		if err != nil {
			return err
		}

		switch state {
		case seedStateComplete:
			return nil
		case seedStatePartial:
			return fmt.Errorf("application tables are partially populated")
		}

		clients, vehicles, ownerships, events, err := loadAndConvert(seeder.dataDir)
		if err != nil {
			return fmt.Errorf("prepare seed data: %w", err)
		}

		if err := tx.CreateInBatches(&clients, batchSize).Error; err != nil {
			return fmt.Errorf("insert clients: %w", err)
		}
		if err := tx.CreateInBatches(&vehicles, batchSize).Error; err != nil {
			return fmt.Errorf("insert vehicles: %w", err)
		}
		if err := tx.CreateInBatches(&ownerships, batchSize).Error; err != nil {
			return fmt.Errorf("insert ownerships: %w", err)
		}
		if err := tx.CreateInBatches(&events, batchSize).Error; err != nil {
			return fmt.Errorf("insert event log: %w", err)
		}

		return nil
	})
}

type seedState int

const (
	seedStateEmpty seedState = iota
	seedStateComplete
	seedStatePartial
)

func inspectState(tx *gorm.DB) (seedState, error) {
	models := []any{&model.Client{}, &model.Vehicle{}, &model.CarOwnership{}, &model.EventLog{}}
	nonEmpty := 0
	for _, item := range models {
		var count int64
		if err := tx.Model(item).Count(&count).Error; err != nil {
			return seedStateEmpty, fmt.Errorf("inspect seed state: %w", err)
		}
		if count > 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return seedStateEmpty, nil
	}
	if nonEmpty == len(models) {
		return seedStateComplete, nil
	}
	return seedStatePartial, nil
}

func loadAndConvert(dataDir string) ([]model.Client, []model.Vehicle, []model.CarOwnership, []model.EventLog, error) {
	var rawClients []rawClient
	var rawCars []rawCar
	var rawServices []rawService
	if err := readJSON(filepath.Join(dataDir, "clients.json"), &rawClients); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := readJSON(filepath.Join(dataDir, "cars.json"), &rawCars); err != nil {
		return nil, nil, nil, nil, err
	}
	if err := readJSON(filepath.Join(dataDir, "services.json"), &rawServices); err != nil {
		return nil, nil, nil, nil, err
	}

	now := time.Now().UTC()
	clients, clientIDs, err := convertClients(rawClients, now)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	vehicles, ownerships, ownershipByKey, err := convertCars(rawCars, clientIDs, now)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	events, err := convertServices(rawServices, ownershipByKey, now)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return clients, vehicles, ownerships, events, nil
}

func readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func convertClients(source []rawClient, now time.Time) ([]model.Client, map[string]struct{}, error) {
	clients := make([]model.Client, 0, len(source))
	ids := make(map[string]struct{}, len(source))
	personalIDs := make(map[string]struct{}, len(source))
	for index, item := range source {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		personalID := strings.TrimSpace(item.IDCard)
		if id == "" || name == "" || personalID == "" {
			return nil, nil, fmt.Errorf("client row %d has an empty required value", index)
		}
		if _, exists := ids[id]; exists {
			return nil, nil, fmt.Errorf("duplicate client id %q", id)
		}
		if _, exists := personalIDs[personalID]; exists {
			return nil, nil, fmt.Errorf("duplicate client personal id %q", personalID)
		}
		ids[id] = struct{}{}
		personalIDs[personalID] = struct{}{}
		clients = append(clients, model.Client{
			ID: id, Name: name, PersonalID: personalID, IsActive: true,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	return clients, ids, nil
}

type convertedCar struct {
	vehicle  model.Vehicle
	clientID string
}

func convertCars(source []rawCar, clientIDs map[string]struct{}, now time.Time) ([]model.Vehicle, []model.CarOwnership, map[string]int, error) {
	converted := make([]convertedCar, 0, len(source))
	vehicleIDs := make(map[int]struct{}, len(source))
	for index, item := range source {
		id, err := positiveInt(item.ID, "car id")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("car row %d: %w", index, err)
		}
		if _, exists := vehicleIDs[id]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate vehicle id %d", id)
		}
		vehicleIDs[id] = struct{}{}
		clientID := strings.TrimSpace(item.ClientID)
		if _, exists := clientIDs[clientID]; !exists {
			return nil, nil, nil, fmt.Errorf("car %d refers to unknown client %q", id, clientID)
		}
		registeredAt, err := time.ParseInLocation(sourceTimeForm, strings.TrimSpace(item.Registered), time.UTC)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("car %d registered timestamp: %w", id, err)
		}
		ownBrand, err := sourceBool(item.OwnBrand)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("car %d ownbrand: %w", id, err)
		}
		accidents, err := nonNegativeInt(item.Accident, "accident count")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("car %d: %w", id, err)
		}
		vehicleType := strings.TrimSpace(item.Type)
		if vehicleType == "" {
			return nil, nil, nil, fmt.Errorf("car %d has an empty type", id)
		}
		converted = append(converted, convertedCar{clientID: clientID, vehicle: model.Vehicle{
			ID: id, Type: vehicleType, RegisteredAt: registeredAt, OwnBrand: ownBrand,
			AccidentCount: accidents, IsActive: true, CreatedAt: now, UpdatedAt: now,
		}})
	}

	sort.Slice(converted, func(leftIndex, rightIndex int) bool {
		return converted[leftIndex].vehicle.ID < converted[rightIndex].vehicle.ID
	})
	vehicles := make([]model.Vehicle, 0, len(converted))
	ownerships := make([]model.CarOwnership, 0, len(converted))
	ownershipByKey := make(map[string]int, len(converted))
	clientOrdinals := make(map[string]int)
	for index, item := range converted {
		clientOrdinals[item.clientID]++
		ordinal := clientOrdinals[item.clientID]
		ownershipID := index + 1
		key := ownershipKey(item.clientID, ordinal)
		if _, exists := ownershipByKey[key]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate ownership mapping for client %q car %d", item.clientID, ordinal)
		}
		ownershipByKey[key] = ownershipID
		vehicles = append(vehicles, item.vehicle)
		ownerships = append(ownerships, model.CarOwnership{
			ID: ownershipID, ClientID: item.clientID, VehicleID: item.vehicle.ID,
			ClientCarNumber: ordinal, IsActive: true, CreatedAt: now, UpdatedAt: now,
		})
	}
	return vehicles, ownerships, ownershipByKey, nil
}

func convertServices(source []rawService, ownershipByKey map[string]int, now time.Time) ([]model.EventLog, error) {
	events := make([]model.EventLog, 0, len(source))
	ids := make(map[int]struct{}, len(source))
	pairs := make(map[string]struct{}, len(source))
	for index, item := range source {
		id, err := positiveInt(item.ID, "service id")
		if err != nil {
			return nil, fmt.Errorf("service row %d: %w", index, err)
		}
		if _, exists := ids[id]; exists {
			return nil, fmt.Errorf("duplicate service id %d", id)
		}
		ids[id] = struct{}{}
		ordinal, err := positiveInt(item.CarID, "client car number")
		if err != nil {
			return nil, fmt.Errorf("service %d: %w", id, err)
		}
		clientID := strings.TrimSpace(item.ClientID)
		ownershipID, exists := ownershipByKey[ownershipKey(clientID, ordinal)]
		if !exists {
			return nil, fmt.Errorf("service %d cannot resolve client %q car %d", id, clientID, ordinal)
		}
		logNumber, err := positiveInt(item.LogNumber, "log number")
		if err != nil {
			return nil, fmt.Errorf("service %d: %w", id, err)
		}
		pair := fmt.Sprintf("%d:%d", ownershipID, logNumber)
		if _, exists := pairs[pair]; exists {
			return nil, fmt.Errorf("duplicate log number %d for ownership %d", logNumber, ownershipID)
		}
		pairs[pair] = struct{}{}
		eventName := strings.TrimSpace(item.Event)
		if eventName == "" {
			return nil, fmt.Errorf("service %d has an empty event", id)
		}
		eventTime, err := optionalTime(item.EventTime)
		if err != nil {
			return nil, fmt.Errorf("service %d event timestamp: %w", id, err)
		}
		events = append(events, model.EventLog{
			ID: id, OwnershipID: ownershipID, LogNumber: logNumber, Event: eventName,
			EventTime: eventTime, DocumentID: optionalDocument(item.DocumentID), CreatedAt: now,
		})
	}
	return events, nil
}

func positiveInt(raw, label string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return value, nil
}

func nonNegativeInt(raw, label string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", label)
	}
	return value, nil
}

func sourceBool(raw string) (bool, error) {
	switch strings.TrimSpace(raw) {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected 0 or 1")
	}
}

func optionalTime(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := time.ParseInLocation(sourceTimeForm, strings.TrimSpace(*raw), time.UTC)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func optionalDocument(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	switch strings.ToLower(value) {
	case "", "0", "null", "no data", "nincs adat":
		return nil
	default:
		return &value
	}
}

func ownershipKey(clientID string, ordinal int) string {
	return fmt.Sprintf("%s\x00%d", clientID, ordinal)
}
