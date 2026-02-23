package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"capim-test/internal/service"
)

type Handler struct {
	service *service.Service
}

type ProblemDetails struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

const (
	problemContentType      = "application/problem+json"
	problemTypeValidation   = "https://capim.test/problems/validation-error"
	problemTypeNotFound     = "https://capim.test/problems/not-found"
	problemTypeConflict     = "https://capim.test/problems/conflict"
	problemTypeUnauthorized = "https://capim.test/problems/unauthorized"
	problemTypeInternal     = "https://capim.test/problems/internal-error"
	problemTypeInvalidParam = "https://capim.test/problems/invalid-parameter"
)

const (
	defaultCursorLimit = 20
	maxCursorLimit     = 100
)

const (
	headerPageLimit  = "X-Page-Limit"
	headerNextCursor = "X-Next-Cursor"
	headerRequestID  = "X-Request-ID"
)

func NewRouter(service *service.Service, serviceName string) *gin.Engine {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "capim-test-api"
	}

	router := gin.New()
	h := &Handler{service: service}
	requestObsMiddleware := requestObservabilityMiddleware(slog.Default())
	router.Use(
		requestid.New(),
		panicRecoveryMiddleware(slog.Default()),
		otelgin.Middleware(serviceName),
		requestObsMiddleware,
	)

	api := router.Group("/api")
	v1 := api.Group("/v1")

	v1.GET("/health", h.health)
	v1.POST("/auth/login", h.login)

	protected := v1.Group("")
	protected.Use(h.requireAuth())
	protected.GET("/clinics", h.listClinics)
	protected.POST("/clinics", h.createClinic)
	protected.GET("/clinics/:id", h.getClinic)
	protected.PATCH("/clinics/:id", h.updateClinic)
	protected.DELETE("/clinics/:id", h.deleteClinic)
	protected.POST("/clinics/:id/dentists", h.createDentist)
	protected.GET("/clinics/:id/dentists", h.listClinicDentists)
	protected.PATCH("/clinics/:id/dentists/:dentist_id", h.updateClinicDentistRole)
	protected.DELETE("/clinics/:id/dentists/:dentist_id", h.unlinkDentistFromClinic)
	protected.PATCH("/dentists/:id", h.updateDentist)
	protected.DELETE("/dentists/:id", h.deleteDentist)

	return router
}

func requestObservabilityMiddleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	meter := otel.Meter("capim-test/http")
	requestCounter, requestCounterErr := meter.Int64Counter(
		"capim.http.server.request.count",
		metric.WithDescription("Total de requests HTTP processadas pela API"),
	)
	if requestCounterErr != nil {
		logger.Error("create request counter", "error", requestCounterErr)
	}

	requestDuration, requestDurationErr := meter.Float64Histogram(
		"capim.http.server.request.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("Duracao de requests HTTP em milissegundos"),
	)
	if requestDurationErr != nil {
		logger.Error("create request duration histogram", "error", requestDurationErr)
	}
	internalErrorCounter, internalErrorCounterErr := meter.Int64Counter(
		"capim.http.server.internal_error.count",
		metric.WithDescription("Total de erros internos HTTP (5xx)"),
	)
	if internalErrorCounterErr != nil {
		logger.Error("create internal error counter", "error", internalErrorCounterErr)
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := c.Writer.Status()
		durationMs := float64(time.Since(start)) / float64(time.Millisecond)
		requestID := c.Writer.Header().Get(headerRequestID)

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		}
		if requestCounter != nil {
			requestCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
		}
		if requestDuration != nil {
			requestDuration.Record(c.Request.Context(), durationMs, metric.WithAttributes(attrs...))
		}

		logAttrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", route,
			"status", status,
			"duration_ms", durationMs,
			"request_id", requestID,
			"client_ip", c.ClientIP(),
		}
		spanContext := trace.SpanFromContext(c.Request.Context()).SpanContext()
		if spanContext.IsValid() {
			logAttrs = append(
				logAttrs,
				"trace_id", spanContext.TraceID().String(),
				"span_id", spanContext.SpanID().String(),
			)
		}
		if len(c.Errors) > 0 {
			lastErr := c.Errors.Last().Err
			logAttrs = append(
				logAttrs,
				"error", lastErr.Error(),
				"error_type", classifyErrorType(lastErr),
			)
		}
		if status >= http.StatusInternalServerError && internalErrorCounter != nil {
			internalAttrs := append([]attribute.KeyValue{}, attrs...)
			if len(c.Errors) > 0 {
				lastErr := c.Errors.Last().Err
				internalAttrs = append(internalAttrs, attribute.String("error.type", classifyErrorType(lastErr)))
			} else {
				internalAttrs = append(internalAttrs, attribute.String("error.type", "unknown"))
			}
			internalErrorCounter.Add(c.Request.Context(), 1, metric.WithAttributes(internalAttrs...))
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.ErrorContext(c.Request.Context(), "http request", logAttrs...)
		case status >= http.StatusBadRequest:
			logger.WarnContext(c.Request.Context(), "http request", logAttrs...)
		default:
			logger.InfoContext(c.Request.Context(), "http request", logAttrs...)
		}
	}
}

func panicRecoveryMiddleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			err := fmt.Errorf("panic recovered: %v", recovered)
			_ = c.Error(err)

			span := trace.SpanFromContext(c.Request.Context())
			if span.SpanContext().IsValid() {
				span.RecordError(err)
				span.SetStatus(codes.Error, "panic recovered")
				span.SetAttributes(
					attribute.Bool("error", true),
					attribute.String("error.type", "panic"),
				)
			}

			requestID := requestid.Get(c)
			logAttrs := []any{
				"panic", recovered,
				"stack_trace", string(debug.Stack()),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"request_id", requestID,
				"client_ip", c.ClientIP(),
			}
			spanContext := span.SpanContext()
			if spanContext.IsValid() {
				logAttrs = append(
					logAttrs,
					"trace_id", spanContext.TraceID().String(),
					"span_id", spanContext.SpanID().String(),
				)
			}
			logger.ErrorContext(c.Request.Context(), "panic recovered", logAttrs...)

			writeProblemResponse(c, http.StatusInternalServerError, problemTypeInternal, "Internal Server Error", "internal server error")
		}()

		c.Next()
	}
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	output, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *Handler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawAuthorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if rawAuthorization == "" {
			h.writeProblem(c, http.StatusUnauthorized, problemTypeUnauthorized, "Unauthorized", "missing bearer token")
			return
		}

		prefix := "Bearer "
		if !strings.HasPrefix(rawAuthorization, prefix) {
			h.writeProblem(c, http.StatusUnauthorized, problemTypeUnauthorized, "Unauthorized", "invalid authorization header")
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(rawAuthorization, prefix))
		if err := h.service.ValidateAccessToken(token); err != nil {
			h.writeProblem(c, http.StatusUnauthorized, problemTypeUnauthorized, "Unauthorized", "invalid token")
			return
		}

		c.Next()
	}
}

func (h *Handler) listClinics(c *gin.Context) {
	limit, cursor, err := parseCursorPagination(c)
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	clinics, nextCursor, err := h.service.ListClinicsWithCursor(c.Request.Context(), limit, cursor)
	if err != nil {
		h.writeError(c, err)
		return
	}

	setCursorHeaders(c, limit, nextCursor)
	c.JSON(http.StatusOK, clinics)
}

func (h *Handler) createClinic(c *gin.Context) {
	var input service.CreateClinicInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	clinic, err := h.service.CreateClinic(c.Request.Context(), input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, clinic)
}

func (h *Handler) getClinic(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	clinic, err := h.service.GetClinic(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, clinic)
}

func (h *Handler) updateClinic(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	var input service.UpdateClinicInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	clinic, err := h.service.UpdateClinic(c.Request.Context(), id, input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, clinic)
}

func (h *Handler) deleteClinic(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	if err := h.service.DeleteClinic(c.Request.Context(), id); err != nil {
		h.writeError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) createDentist(c *gin.Context) {
	clinicID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	var input service.CreateDentistInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	dentist, created, err := h.service.CreateOrAttachDentist(c.Request.Context(), clinicID, input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	if created {
		c.JSON(http.StatusCreated, dentist)
		return
	}
	c.JSON(http.StatusOK, dentist)
}

func (h *Handler) listClinicDentists(c *gin.Context) {
	clinicID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	limit, cursor, err := parseCursorPagination(c)
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	dentists, nextCursor, err := h.service.ListClinicDentistsWithCursor(c.Request.Context(), clinicID, limit, cursor)
	if err != nil {
		h.writeError(c, err)
		return
	}

	setCursorHeaders(c, limit, nextCursor)
	c.JSON(http.StatusOK, dentists)
}

func (h *Handler) updateClinicDentistRole(c *gin.Context) {
	clinicID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	dentistID, err := parseID(c, "dentist_id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	var input service.UpdateClinicDentistRoleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	dentist, err := h.service.UpdateClinicDentistRole(c.Request.Context(), clinicID, dentistID, input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, dentist)
}

func (h *Handler) unlinkDentistFromClinic(c *gin.Context) {
	clinicID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	dentistID, err := parseID(c, "dentist_id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	if err := h.service.UnlinkDentistFromClinic(c.Request.Context(), clinicID, dentistID); err != nil {
		h.writeError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) updateDentist(c *gin.Context) {
	dentistID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	var input service.UpdateDentistInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", fmt.Sprintf("invalid request body: %s", err.Error()))
		return
	}

	dentist, err := h.service.UpdateDentist(c.Request.Context(), dentistID, input)
	if err != nil {
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, dentist)
}

func (h *Handler) deleteDentist(c *gin.Context) {
	dentistID, err := parseID(c, "id")
	if err != nil {
		h.writeProblem(c, http.StatusBadRequest, problemTypeInvalidParam, "Invalid Parameter", err.Error())
		return
	}

	if err := h.service.DeleteDentist(c.Request.Context(), dentistID); err != nil {
		h.writeError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		h.writeProblem(c, http.StatusBadRequest, problemTypeValidation, "Validation Error", err.Error())
	case errors.Is(err, service.ErrNotFound):
		h.writeProblem(c, http.StatusNotFound, problemTypeNotFound, "Not Found", err.Error())
	case errors.Is(err, service.ErrConflict):
		h.writeProblem(c, http.StatusConflict, problemTypeConflict, "Conflict", err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		h.writeProblem(c, http.StatusUnauthorized, problemTypeUnauthorized, "Unauthorized", err.Error())
	default:
		_ = c.Error(err)
		span := trace.SpanFromContext(c.Request.Context())
		spanContext := span.SpanContext()
		if span.SpanContext().IsValid() {
			span.RecordError(err)
			span.SetStatus(codes.Error, "internal server error")
			span.SetAttributes(
				attribute.Bool("error", true),
				attribute.String("error.type", classifyErrorType(err)),
			)
		}
		logAttrs := []any{
			"error", err.Error(),
			"error_type", classifyErrorType(err),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"request_id", requestid.Get(c),
		}
		if spanContext.IsValid() {
			logAttrs = append(
				logAttrs,
				"trace_id", spanContext.TraceID().String(),
				"span_id", spanContext.SpanID().String(),
			)
		}
		slog.ErrorContext(c.Request.Context(), "internal server error", logAttrs...)
		h.writeProblem(c, http.StatusInternalServerError, problemTypeInternal, "Internal Server Error", "internal server error")
	}
}

func (h *Handler) writeProblem(c *gin.Context, status int, problemType string, title string, detail string) {
	writeProblemResponse(c, status, problemType, title, detail)
}

func writeProblemResponse(c *gin.Context, status int, problemType string, title string, detail string) {
	if problemType == "" {
		problemType = "about:blank"
	}
	if title == "" {
		title = http.StatusText(status)
	}

	requestID := requestid.Get(c)
	if requestID != "" {
		c.Header(headerRequestID, requestID)
	}

	c.Header("Content-Type", problemContentType)
	c.AbortWithStatusJSON(status, ProblemDetails{
		Type:      problemType,
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  c.Request.URL.Path,
		RequestID: requestID,
	})
}

func classifyErrorType(err error) string {
	if err == nil {
		return "unknown"
	}
	root := err
	for {
		unwrapped := errors.Unwrap(root)
		if unwrapped == nil {
			break
		}
		root = unwrapped
	}
	return fmt.Sprintf("%T", root)
}

func parseID(c *gin.Context, param string) (string, error) {
	id := strings.TrimSpace(c.Param(param))
	if id == "" {
		return "", fmt.Errorf("invalid parameter %q: must be a UUIDv7", param)
	}
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.Version() != 7 {
		return "", fmt.Errorf("invalid parameter %q: must be a UUIDv7", param)
	}
	return parsed.String(), nil
}

func parseCursorPagination(c *gin.Context) (int, *string, error) {
	limit := defaultCursorLimit
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return 0, nil, fmt.Errorf("invalid parameter %q: must be an integer between 1 and %d", "limit", maxCursorLimit)
		}
		if parsedLimit < 1 || parsedLimit > maxCursorLimit {
			return 0, nil, fmt.Errorf("invalid parameter %q: must be between 1 and %d", "limit", maxCursorLimit)
		}
		limit = parsedLimit
	}

	rawCursor := strings.TrimSpace(c.Query("cursor"))
	if rawCursor == "" {
		return limit, nil, nil
	}

	parsedCursor, err := uuid.Parse(rawCursor)
	if err != nil || parsedCursor.Version() != 7 {
		return 0, nil, fmt.Errorf("invalid parameter %q: must be a UUIDv7", "cursor")
	}

	cursor := parsedCursor.String()
	return limit, &cursor, nil
}

func setCursorHeaders(c *gin.Context, limit int, nextCursor *string) {
	c.Header(headerPageLimit, strconv.Itoa(limit))
	c.Header(headerNextCursor, "")
	c.Header("Link", "")

	if nextCursor == nil || strings.TrimSpace(*nextCursor) == "" {
		return
	}

	c.Header(headerNextCursor, *nextCursor)
	nextURL := buildNextPageURL(c, *nextCursor, limit)
	if nextURL != "" {
		c.Header("Link", fmt.Sprintf("<%s>; rel=\"next\"", nextURL))
	}
}

func buildNextPageURL(c *gin.Context, nextCursor string, limit int) string {
	u := &url.URL{Path: c.Request.URL.Path}
	query := c.Request.URL.Query()
	query.Set("cursor", nextCursor)
	query.Set("limit", strconv.Itoa(limit))
	u.RawQuery = query.Encode()
	return u.String()
}
