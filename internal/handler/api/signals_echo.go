package api

import (
    "net/http"
    "time"

    models "FinPull/internal/domain/models"
    domrepo "FinPull/internal/domain/repository"
    "FinPull/internal/usecase"
    xhttp "FinPull/pkg/http"
    xlogger "FinPull/pkg/logger"

    "github.com/labstack/echo/v4"
)

// SignalsEchoHandler implements Echo-based HTTP handlers following Clean Architecture.
type SignalsEchoHandler struct {
	logger *xlogger.Logger
	agg    *usecase.SignalAggregator
}

func NewSignalsEchoHandler(logger *xlogger.Logger, agg *usecase.SignalAggregator) *SignalsEchoHandler {
	return &SignalsEchoHandler{logger: logger, agg: agg}
}

func (h *SignalsEchoHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api")
	g.GET("/regime", h.Regime)
	g.GET("/vol", h.Vol)
	g.GET("/anomaly", h.Anomaly)
	g.GET("/edge", h.Edge)
}

func (h *SignalsEchoHandler) Regime(c echo.Context) error {
	start := time.Now()
	req := &models.RegimeRequest{}
	if verr := xhttp.ReadAndValidateRequest(c, req); verr != nil {
		return xhttp.BadRequestResponse(c, verr)
	}
	tf := domrepo.NormalizeTimeframe(req.TF)

	res, err := h.agg.LatestRegime(c.Request().Context(), req.Symbol, req.N, tf)
	if err != nil {
		h.logger.Error("regime usecase error", xlogger.Error(err))
		return xhttp.AppErrorResponse(c, err)
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "private, max-age=15")
	_ = c.Response().Header().Write
	_ = start
	return xhttp.SuccessResponse(c, res)
}

func (h *SignalsEchoHandler) Vol(c echo.Context) error {
	req := &models.VolRequest{}
	if verr := xhttp.ReadAndValidateRequest(c, req); verr != nil {
		return xhttp.BadRequestResponse(c, verr)
	}
	tf := domrepo.NormalizeTimeframe(req.TF)

	res, err := h.agg.VolForecast(c.Request().Context(), req.Symbol, req.Horizon, req.N, tf)
	if err != nil {
		h.logger.Error("vol usecase error", xlogger.Error(err))
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, res)
}

func (h *SignalsEchoHandler) Anomaly(c echo.Context) error {
	req := &models.AnomalyRequest{}
	if verr := xhttp.ReadAndValidateRequest(c, req); verr != nil {
		return xhttp.BadRequestResponse(c, verr)
	}
	tf := domrepo.NormalizeTimeframe(req.TF)

	res, err := h.agg.Anomalies(c.Request().Context(), req.Symbol, req.N, tf)
	if err != nil {
		h.logger.Error("anomaly usecase error", xlogger.Error(err))
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, res)
}

func (h *SignalsEchoHandler) Edge(c echo.Context) error {
	req := &models.EdgeRequest{}
	if verr := xhttp.ReadAndValidateRequest(c, req); verr != nil {
		return xhttp.BadRequestResponse(c, verr)
	}
	tf := domrepo.NormalizeTimeframe(req.TF)

	res, err := h.agg.Edge(c.Request().Context(), req.Symbol, req.Horizon, req.N, tf)
	if err != nil {
		h.logger.Error("edge usecase error", xlogger.Error(err))
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, res)
}

// Ensure HTTP status is OK on DataResponse
func init() { _ = http.StatusOK }
