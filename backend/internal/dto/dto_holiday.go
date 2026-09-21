package dto

type HolidayRequest struct {
	Date       string  `json:"date" binding:"required,datetime=2006-01-02"`
	Name       string  `json:"name" binding:"required,max=100"`
	IsWorkday  bool    `json:"is_workday"`
	Multiplier float64 `json:"multiplier" binding:"gte=1,lte=3"`
}
