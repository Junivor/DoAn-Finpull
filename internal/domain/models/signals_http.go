package models

// Requests for analytics HTTP endpoints. Defined in domain for consistency and reuse.

type RegimeRequest struct {
	Symbol string `query:"symbol" json:"symbol" validate:"required"`
	N      int    `query:"n" json:"n" default:"600" validate:"gte=1,lte=5000"`
	TF     string `query:"tf" json:"tf" default:"1m" validate:"oneof=1s 1m 5m"`
}

type VolRequest struct {
	Symbol  string `query:"symbol" json:"symbol" validate:"required"`
	Horizon string `query:"horizon" json:"horizon" default:"5m" validate:"oneof=1m 5m 15m 30m 1h"`
	N       int    `query:"n" json:"n" default:"600" validate:"gte=1,lte=5000"`
	TF      string `query:"tf" json:"tf" default:"1m" validate:"oneof=1s 1m 5m"`
}

type AnomalyRequest struct {
	Symbol string `query:"symbol" json:"symbol" validate:"required"`
	N      int    `query:"n" json:"n" default:"1200" validate:"gte=1,lte=10000"`
	TF     string `query:"tf" json:"tf" default:"1m" validate:"oneof=1s 1m 5m"`
}

type EdgeRequest struct {
	Symbol  string `query:"symbol" json:"symbol" validate:"required"`
	Horizon string `query:"horizon" json:"horizon" default:"15m" validate:"oneof=5m 15m 30m 1h"`
	N       int    `query:"n" json:"n" default:"600" validate:"gte=1,lte=5000"`
	TF      string `query:"tf" json:"tf" default:"1m" validate:"oneof=1s 1m 5m"`
}
