package entity

type SessionRequest struct {
	FieldID     string `json:"fieldId"`
	IsIndoor    string `json:"isIndoor"`
	PlaceTypeID string `json:"placeTypeId"`
	SearchDate  string `json:"searchDate"`
	SportTypeID string `json:"sportTypeId"`
}

type Session struct {
	ID             string `json:"id"`
	OpenDate       string `json:"openDate"`
	OpenStartTime  string `json:"openStartTime"`
	OpenEndTime    string `json:"openEndTime"`
	PlaceName      string `json:"placeName"`
	SessionsStatus string `json:"sessionsStatus"`
}

type ReserveRequest struct {
	Number        int              `json:"number"`
	OrderUseDate  string           `json:"orderUseDate"`
	RequestsList  []ReserveSession `json:"requestsList"`
	FieldName     string           `json:"fieldName"`
	FieldID       string           `json:"fieldId"`
	SiteName      string           `json:"siteName"`
	SportTypeName string           `json:"sportTypeName"`
	SportTypeID   string           `json:"sportTypeId"`
}

type ReserveSession struct {
	SessionsID string `json:"sessionsId"`
}

type GetSessionIDRequest struct {
	Date      string   `json:"date"`
	TimeSlots []string `json:"timeSlots"`
	CourtNums []string `json:"courtNums"`
}

const Second = 400000000
