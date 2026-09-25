package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConvertSourceData(testContext *testing.T) {
	now := time.Date(2026, time.September, 25, 10, 30, 0, 0, time.UTC)
	clients, clientIDs, err := convertClients([]rawClient{
		{ID: " client-a ", Name: " Alice Example ", IDCard: " AA11 "},
		{ID: "client-b", Name: "Bob Example", IDCard: "BB22"},
	}, now)
	if err != nil {
		testContext.Fatalf("convertClients returned an error: %v", err)
	}
	if len(clients) != 2 || clients[0].ID != "client-a" || clients[0].Name != "Alice Example" || clients[0].PersonalID != "AA11" {
		testContext.Fatalf("unexpected clients: %+v", clients)
	}
	if !clients[0].IsActive || !clients[0].CreatedAt.Equal(now) || !clients[0].UpdatedAt.Equal(now) {
		testContext.Errorf("client state or timestamps were not initialized: %+v", clients[0])
	}
	if _, exists := clientIDs["client-a"]; !exists {
		testContext.Fatalf("trimmed client ID is absent from lookup: %#v", clientIDs)
	}

	vehicles, ownerships, ownershipByKey, err := convertCars([]rawCar{
		validRawCar("20", "client-a", "SUV", "1", "2"),
		validRawCar("30", "client-b", "Van", "0", "0"),
		validRawCar("10", "client-a", "Sedan", "0", "1"),
	}, clientIDs, now)
	if err != nil {
		testContext.Fatalf("convertCars returned an error: %v", err)
	}
	if len(vehicles) != 3 {
		testContext.Fatalf("vehicle count = %d, want 3", len(vehicles))
	}
	if got := []int{vehicles[0].ID, vehicles[1].ID, vehicles[2].ID}; !reflect.DeepEqual(got, []int{10, 20, 30}) {
		testContext.Fatalf("vehicles are not deterministically sorted: %v", got)
	}
	if vehicles[0].OwnBrand || !vehicles[1].OwnBrand || vehicles[1].AccidentCount != 2 {
		testContext.Errorf("vehicle values were not converted: %+v", vehicles)
	}
	if !vehicles[0].RegisteredAt.Equal(time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)) {
		testContext.Errorf("unexpected registration time: %s", vehicles[0].RegisteredAt)
	}
	if len(ownerships) != 3 {
		testContext.Fatalf("ownership count = %d, want 3", len(ownerships))
	}
	if ownerships[0].ClientID != "client-a" || ownerships[0].VehicleID != 10 || ownerships[0].ClientCarNumber != 1 {
		testContext.Errorf("unexpected first ownership: %+v", ownerships[0])
	}
	if ownerships[1].ClientID != "client-a" || ownerships[1].VehicleID != 20 || ownerships[1].ClientCarNumber != 2 {
		testContext.Errorf("unexpected second ownership: %+v", ownerships[1])
	}
	if ownerships[2].ClientID != "client-b" || ownerships[2].VehicleID != 30 || ownerships[2].ClientCarNumber != 1 {
		testContext.Errorf("unexpected third ownership: %+v", ownerships[2])
	}

	eventTime := "2021-02-03 04:05:06"
	document := " DOC-42 "
	missingDocument := "0"
	events, err := convertServices([]rawService{
		{
			ID: "100", ClientID: "client-a", CarID: "2", LogNumber: "1",
			Event: " service ", EventTime: &eventTime, DocumentID: &document,
		},
		{
			ID: "101", ClientID: "client-a", CarID: "1", LogNumber: "1",
			Event: "registered", DocumentID: &missingDocument,
		},
	}, ownershipByKey, now)
	if err != nil {
		testContext.Fatalf("convertServices returned an error: %v", err)
	}
	if len(events) != 2 || events[0].OwnershipID != ownershipByKey[ownershipKey("client-a", 2)] {
		testContext.Fatalf("service was assigned to the wrong ownership: %+v", events)
	}
	if events[0].Event != "service" || events[0].EventTime == nil || events[0].DocumentID == nil || *events[0].DocumentID != "DOC-42" {
		testContext.Errorf("service values were not normalized: %+v", events[0])
	}
	if want := time.Date(2021, time.February, 3, 4, 5, 6, 0, time.UTC); !events[0].EventTime.Equal(want) {
		testContext.Errorf("event time = %s, want %s", events[0].EventTime, want)
	}
	if events[1].EventTime != nil || events[1].DocumentID != nil {
		testContext.Errorf("missing event values should remain nil: %+v", events[1])
	}
	if !events[0].CreatedAt.Equal(now) || !events[1].CreatedAt.Equal(now) {
		testContext.Errorf("event creation timestamps were not initialized")
	}
}

func TestConvertClientsRejectsInvalidData(testContext *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name        string
		source      []rawClient
		wantInError string
	}{
		{
			name:        "empty required value",
			source:      []rawClient{{ID: "1", Name: " ", IDCard: "A1"}},
			wantInError: "empty required value",
		},
		{
			name:        "duplicate client id",
			source:      []rawClient{{ID: "1", Name: "A", IDCard: "A1"}, {ID: " 1 ", Name: "B", IDCard: "B1"}},
			wantInError: "duplicate client id",
		},
		{
			name:        "duplicate personal id",
			source:      []rawClient{{ID: "1", Name: "A", IDCard: "A1"}, {ID: "2", Name: "B", IDCard: " A1 "}},
			wantInError: "duplicate client personal id",
		},
	}

	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			_, _, err := convertClients(test.source, now)
			assertErrorContains(testContext, err, test.wantInError)
		})
	}
}

func TestConvertCarsRejectsInvalidData(testContext *testing.T) {
	now := time.Now().UTC()
	clientIDs := map[string]struct{}{"client": {}}
	invalidTimestamp := validRawCar("1", "client", "Sedan", "0", "0")
	invalidTimestamp.Registered = "not-a-time"
	invalidBoolean := validRawCar("1", "client", "Sedan", "yes", "0")
	emptyType := validRawCar("1", "client", " ", "0", "0")

	tests := []struct {
		name        string
		source      []rawCar
		wantInError string
	}{
		{name: "invalid vehicle id", source: []rawCar{validRawCar("0", "client", "Sedan", "0", "0")}, wantInError: "car id must be a positive integer"},
		{name: "duplicate vehicle id", source: []rawCar{validRawCar("1", "client", "Sedan", "0", "0"), validRawCar("1", "client", "SUV", "1", "0")}, wantInError: "duplicate vehicle id"},
		{name: "unknown client", source: []rawCar{validRawCar("1", "missing", "Sedan", "0", "0")}, wantInError: "unknown client"},
		{name: "invalid timestamp", source: []rawCar{invalidTimestamp}, wantInError: "registered timestamp"},
		{name: "invalid boolean", source: []rawCar{invalidBoolean}, wantInError: "ownbrand"},
		{name: "negative accident count", source: []rawCar{validRawCar("1", "client", "Sedan", "0", "-1")}, wantInError: "accident count must be a non-negative integer"},
		{name: "empty vehicle type", source: []rawCar{emptyType}, wantInError: "empty type"},
	}

	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			_, _, _, err := convertCars(test.source, clientIDs, now)
			assertErrorContains(testContext, err, test.wantInError)
		})
	}
}

func TestConvertServicesRejectsInvalidData(testContext *testing.T) {
	now := time.Now().UTC()
	ownerships := map[string]int{ownershipKey("client", 1): 7}
	invalidTime := "not-a-time"
	emptyEvent := validRawService("1", "client", "1", "1")
	emptyEvent.Event = " "
	badTime := validRawService("1", "client", "1", "1")
	badTime.EventTime = &invalidTime

	tests := []struct {
		name        string
		source      []rawService
		wantInError string
	}{
		{name: "invalid service id", source: []rawService{validRawService("0", "client", "1", "1")}, wantInError: "service id must be a positive integer"},
		{name: "duplicate service id", source: []rawService{validRawService("1", "client", "1", "1"), validRawService("1", "client", "1", "2")}, wantInError: "duplicate service id"},
		{name: "invalid client car number", source: []rawService{validRawService("1", "client", "0", "1")}, wantInError: "client car number must be a positive integer"},
		{name: "unknown ownership", source: []rawService{validRawService("1", "missing", "1", "1")}, wantInError: "cannot resolve client"},
		{name: "invalid log number", source: []rawService{validRawService("1", "client", "1", "0")}, wantInError: "log number must be a positive integer"},
		{name: "duplicate ownership log number", source: []rawService{validRawService("1", "client", "1", "1"), validRawService("2", "client", "1", "1")}, wantInError: "duplicate log number"},
		{name: "empty event", source: []rawService{emptyEvent}, wantInError: "empty event"},
		{name: "invalid event time", source: []rawService{badTime}, wantInError: "event timestamp"},
	}

	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			_, err := convertServices(test.source, ownerships, now)
			assertErrorContains(testContext, err, test.wantInError)
		})
	}
}

func TestOptionalDocumentNormalization(testContext *testing.T) {
	for _, missing := range []string{"", " ", "0", "null", "NULL", "no data", "nincs adat"} {
		testContext.Run("missing_"+strings.ReplaceAll(missing, " ", "_"), func(testContext *testing.T) {
			if document := optionalDocument(&missing); document != nil {
				testContext.Fatalf("optionalDocument(%q) = %q, want nil", missing, *document)
			}
		})
	}

	if document := optionalDocument(nil); document != nil {
		testContext.Fatalf("optionalDocument(nil) = %q, want nil", *document)
	}
	value := " DOC-9 "
	document := optionalDocument(&value)
	if document == nil || *document != "DOC-9" {
		testContext.Fatalf("optionalDocument(%q) = %v, want DOC-9", value, document)
	}
}

func TestLoadAndConvertReadsJSONFiles(testContext *testing.T) {
	directory := testContext.TempDir()
	writeJSONFixture(testContext, directory, "clients.json", []rawClient{{ID: "client", Name: "Example", IDCard: "ID1"}})
	writeJSONFixture(testContext, directory, "cars.json", []rawCar{validRawCar("1", "client", "Sedan", "1", "0")})
	writeJSONFixture(testContext, directory, "services.json", []rawService{validRawService("1", "client", "1", "1")})

	clients, vehicles, ownerships, events, err := loadAndConvert(directory)
	if err != nil {
		testContext.Fatalf("loadAndConvert returned an error: %v", err)
	}
	if len(clients) != 1 || len(vehicles) != 1 || len(ownerships) != 1 || len(events) != 1 {
		testContext.Fatalf("unexpected converted counts: clients=%d vehicles=%d ownerships=%d events=%d", len(clients), len(vehicles), len(ownerships), len(events))
	}

	missingDirectory := testContext.TempDir()
	_, _, _, _, err = loadAndConvert(missingDirectory)
	assertErrorContains(testContext, err, "read")

	malformedDirectory := testContext.TempDir()
	if err := os.WriteFile(filepath.Join(malformedDirectory, "clients.json"), []byte("{"), 0o600); err != nil {
		testContext.Fatalf("write malformed fixture: %v", err)
	}
	_, _, _, _, err = loadAndConvert(malformedDirectory)
	assertErrorContains(testContext, err, "decode")
}

func validRawCar(id, clientID, vehicleType, ownBrand, accident string) rawCar {
	return rawCar{
		ID: id, ClientID: clientID, Type: vehicleType,
		Registered: "2020-01-02 03:04:05", OwnBrand: ownBrand, Accident: accident,
	}
}

func validRawService(id, clientID, carID, logNumber string) rawService {
	return rawService{
		ID: id, ClientID: clientID, CarID: carID,
		LogNumber: logNumber, Event: "service",
	}
}

func writeJSONFixture(testContext *testing.T, directory, name string, value any) {
	testContext.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		testContext.Fatalf("marshal %s fixture: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
		testContext.Fatalf("write %s fixture: %v", name, err)
	}
}

func assertErrorContains(testContext *testing.T, err error, expected string) {
	testContext.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		testContext.Fatalf("error = %v, want error containing %q", err, expected)
	}
}
