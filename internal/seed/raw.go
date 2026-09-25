package seed

type rawClient struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IDCard string `json:"idcard"`
}

type rawCar struct {
	ID         string `json:"id"`
	ClientID   string `json:"client_id"`
	Type       string `json:"type"`
	Registered string `json:"registered"`
	OwnBrand   string `json:"ownbrand"`
	Accident   string `json:"accident"`
}

type rawService struct {
	ID         string  `json:"id"`
	ClientID   string  `json:"client_id"`
	CarID      string  `json:"car_id"`
	LogNumber  string  `json:"lognumber"`
	Event      string  `json:"event"`
	EventTime  *string `json:"eventtime"`
	DocumentID *string `json:"document_id"`
}
