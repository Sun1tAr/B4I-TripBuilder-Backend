package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/service"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	routedto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/route"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type RouteHandler struct {
	routeService *service.RouteService
	log          *logger.Logger
}

func NewRouteHandler(routeService *service.RouteService, log *logger.Logger) *RouteHandler {
	return &RouteHandler{
		routeService: routeService,
		log:          log,
	}
}

// DeleteRoute удаляет маршрут
func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	routeIDStr := chi.URLParam(r, "id")

	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		log.Debug("DeleteRoute: неверный ID маршрута", "route_id", routeIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid route id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	err = h.routeService.DeleteRoute(r.Context(), routeID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRouteNotFound):
			h.writeError(w, http.StatusNotFound, "route not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this route", "")
		default:
			log.Error("Ошибка удаления маршрута", "error", err, "route_id", routeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "route deleted successfully"})
}

// ToggleLike ставит или убирает лайк
func (h *RouteHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	routeIDStr := chi.URLParam(r, "id")

	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		log.Debug("ToggleLike: неверный ID маршрута", "route_id", routeIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid route id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	isLiked, count, err := h.routeService.ToggleLike(r.Context(), routeID, *userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRouteNotFound):
			h.writeError(w, http.StatusNotFound, "route not found", "")
		default:
			log.Error("Ошибка переключения лайка", "error", err, "route_id", routeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked": isLiked,
		"count": count,
	})
}

// GetRouteLikes возвращает список пользователей, лайкнувших маршрут
func (h *RouteHandler) GetRouteLikes(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	routeIDStr := chi.URLParam(r, "id")

	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		log.Debug("GetRouteLikes: неверный ID маршрута", "route_id", routeIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid route id", "")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	userIDs, total, err := h.routeService.GetRouteLikes(r.Context(), routeID, page, size)
	if err != nil {
		log.Error("Ошибка получения списка лайков", "error", err, "route_id", routeID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	response := map[string]interface{}{
		"route_id": routeID,
		"users":    userIDs,
		"total":    total,
		"page":     page,
		"size":     size,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetAllTags возвращает список всех тегов
func (h *RouteHandler) GetAllTags(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetAllTags: запрос списка тегов")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 50
	}

	tags, total, err := h.routeService.GetAllTags(r.Context(), page, size)
	if err != nil {
		log.Error("Ошибка получения тегов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	response := map[string]interface{}{
		"items": tags,
		"total": total,
		"page":  page,
		"size":  size,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetRoutesByTag возвращает маршруты по тегу
func (h *RouteHandler) GetRoutesByTag(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	vars := mux.Vars(r)
	tagName := vars["tag"]

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 10
	}

	userID := h.getUserIDFromContext(r)

	routes, total, err := h.routeService.GetRoutesByTag(r.Context(), tagName, page, size, userID)
	if err != nil {
		log.Error("Ошибка получения маршрутов по тегу", "error", err, "tag", tagName)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]routedto.RouteResponse, len(routes))
	for i, route := range routes {
		items[i] = routedto.RouteResponse{
			ID:          route.ID,
			Title:       route.Title,
			Description: route.Description,
			Country:     route.Country,
			ImageURL:    route.ImageURL,
			UserID:      route.UserID,
			IsPublic:    route.IsPublic,
			TotalDays:   route.TotalDays,
			TotalHours:  route.TotalHours,
			LikesCount:  route.LikesCount,
			EventsCount: route.EventsCount,
			IsLiked:     route.IsLiked,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		}
	}

	totalPages := (int(total) + size - 1) / size

	response := routedto.ListRoutesResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *RouteHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *RouteHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *RouteHandler) getUserIDFromContext(r *http.Request) *uuid.UUID {
	userIDStr, ok := r.Context().Value("user_id").(string)
	if !ok || userIDStr == "" {
		return nil
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.log.Debug("getUserIDFromContext: Неверный формат user_id в контексте", "user_id", userIDStr)
		return nil
	}
	return &userID
}

// transferToResponse преобразует модель перемещения в DTO ответа
func transferToResponse(t *models.Transfer) routedto.TransferResponse {
	resp := routedto.TransferResponse{
		ID:              t.ID,
		FromWaypointID:  t.FromWaypointID,
		ToWaypointID:    t.ToWaypointID,
		DurationHours:   t.DurationHours,
		DurationMinutes: t.DurationMinutes,
		TransportType:   string(t.TransportType),
		OrderIndex:      t.OrderIndex,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
	return resp
}

// routeToResponse преобразует модель маршрута в DTO ответа
func routeToResponse(route *models.RouteWithDetails) *routedto.RouteDetailResponse {
	resp := &routedto.RouteDetailResponse{
		RouteResponse: routedto.RouteResponse{
			ID:          route.ID,
			Title:       route.Title,
			Description: route.Description,
			Country:     route.Country,
			ImageURL:    route.ImageURL,
			UserID:      route.UserID,
			IsPublic:    route.IsPublic,
			TotalDays:   route.TotalDays,
			TotalHours:  route.TotalHours,
			LikesCount:  route.LikesCount,
			EventsCount: route.EventsCount,
			IsLiked:     route.IsLiked,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		},
		Waypoints: make([]routedto.WaypointResponse, len(route.Waypoints)),
		Tags:      make([]routedto.TagResponse, len(route.Tags)),
		Transfers: make([]routedto.TransferResponse, len(route.Transfers)),
	}

	for i, wp := range route.Waypoints {
		resp.Waypoints[i] = routedto.WaypointResponse{
			ID:            wp.ID,
			City:          wp.City,
			Description:   wp.Description,
			Latitude:      wp.Latitude,
			Longitude:     wp.Longitude,
			DurationDays:  wp.DurationDays,
			DurationHours: wp.DurationHours,
			OrderIndex:    wp.OrderIndex,
		}
	}

	for i, tag := range route.Tags {
		resp.Tags[i] = routedto.TagResponse{
			ID:          tag.ID.String(),
			Name:        tag.Name,
			RoutesCount: tag.RoutesCount,
		}
	}

	for i, t := range route.Transfers {
		resp.Transfers[i] = transferToResponse(&t)
	}

	return resp
}

// GetRouteByID возвращает маршрут по ID
func (h *RouteHandler) GetRouteByID(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	routeIDStr := chi.URLParam(r, "id")

	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		log.Debug("GetRouteByID: неверный ID маршрута", "route_id", routeIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid route id", "")
		return
	}

	userID := h.getUserIDFromContext(r)

	route, err := h.routeService.GetRouteByID(r.Context(), routeID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRouteNotFound):
			h.writeError(w, http.StatusNotFound, "route not found", "")
		default:
			log.Error("Ошибка получения маршрута", "error", err, "route_id", routeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	response := routeToResponse(route)
	h.writeJSON(w, http.StatusOK, response)
}

// CreateRoute создаёт новый маршрут
func (h *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("CreateRoute: создание маршрута")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req routedto.CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("CreateRoute: ошибка декодирования запроса", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Логируем количество перемещений
	log.Debug("CreateRoute: данные запроса",
		"title", req.Title,
		"waypoints_count", len(req.Waypoints),
		"transfers_count", len(req.Transfers),
	)

	// Преобразуем DTO в модель
	createReq := &models.CreateRouteRequest{
		Title:       req.Title,
		Description: req.Description,
		Country:     req.Country,
		ImageURL:    req.ImageURL,
		IsPublic:    req.IsPublic,
		Tags:        req.Tags,
		Waypoints:   make([]models.CreateWaypoint, len(req.Waypoints)),
		Transfers:   make([]models.CreateTransferRequest, len(req.Transfers)),
	}

	for i, wp := range req.Waypoints {
		createReq.Waypoints[i] = models.CreateWaypoint{
			City:          wp.City,
			Description:   wp.Description,
			Latitude:      wp.Latitude,
			Longitude:     wp.Longitude,
			DurationDays:  wp.DurationDays,
			DurationHours: wp.DurationHours,
			OrderIndex:    wp.OrderIndex,
		}
	}

	for i, t := range req.Transfers {
		createReq.Transfers[i] = models.CreateTransferRequest{
			FromOrderIndex:  t.FromOrderIndex,
			ToOrderIndex:    t.ToOrderIndex,
			DurationHours:   t.DurationHours,
			DurationMinutes: t.DurationMinutes,
			TransportType:   models.TransportType(t.TransportType),
		}
	}

	route, err := h.routeService.CreateRoute(r.Context(), *userID, createReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTitleTooShort):
			h.writeError(w, http.StatusBadRequest, "title must be at least 3 characters", "")
		case errors.Is(err, service.ErrInvalidWaypoints):
			h.writeError(w, http.StatusBadRequest, "route must have at least one waypoint", "")
		case errors.Is(err, service.ErrInvalidCoordinates):
			h.writeError(w, http.StatusBadRequest, "invalid coordinates", "")
		default:
			// Проверяем ошибки валидации перемещений
			if strings.Contains(err.Error(), "transfers") {
				h.writeError(w, http.StatusBadRequest, "invalid transfers", err.Error())
				return
			}
			log.Error("Ошибка создания маршрута", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	response := routeToResponse(route)
	h.writeJSON(w, http.StatusCreated, response)
}

// UpdateRoute обновляет маршрут
func (h *RouteHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	routeIDStr := chi.URLParam(r, "id")

	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		log.Debug("UpdateRoute: неверный ID маршрута", "route_id", routeIDStr)
		h.writeError(w, http.StatusBadRequest, "invalid route id", "")
		return
	}

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req routedto.UpdateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("UpdateRoute: ошибка декодирования запроса", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	log.Debug("UpdateRoute: данные запроса",
		"route_id", routeID,
		"waypoints_count", len(req.Waypoints),
		"transfers_count", len(req.Transfers),
	)

	// Преобразуем DTO в модель
	updateReq := &models.UpdateRouteRequest{
		Title:       req.Title,
		Description: req.Description,
		Country:     req.Country,
		ImageURL:    req.ImageURL,
		IsPublic:    req.IsPublic,
		Tags:        req.Tags,
	}

	if req.Waypoints != nil {
		updateReq.Waypoints = make([]models.UpdateWaypoint, len(req.Waypoints))
		for i, wp := range req.Waypoints {
			updateReq.Waypoints[i] = models.UpdateWaypoint{
				ID:            wp.ID,
				City:          wp.City,
				Description:   wp.Description,
				Latitude:      wp.Latitude,
				Longitude:     wp.Longitude,
				DurationDays:  wp.DurationDays,
				DurationHours: wp.DurationHours,
				OrderIndex:    wp.OrderIndex,
			}
		}
	}

	if req.Transfers != nil {
		updateReq.Transfers = make([]models.UpdateTransferRequest, len(req.Transfers))
		for i, t := range req.Transfers {
			var transportType *models.TransportType
			if t.TransportType != nil {
				tt := models.TransportType(*t.TransportType)
				transportType = &tt
			}

			updateReq.Transfers[i] = models.UpdateTransferRequest{
				ID:              t.ID,
				FromOrderIndex:  t.FromOrderIndex,
				ToOrderIndex:    t.ToOrderIndex,
				DurationHours:   t.DurationHours,
				DurationMinutes: t.DurationMinutes,
				TransportType:   transportType,
			}
		}
	}

	route, err := h.routeService.UpdateRoute(r.Context(), routeID, *userID, updateReq)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRouteNotFound):
			h.writeError(w, http.StatusNotFound, "route not found", "")
		case errors.Is(err, service.ErrNotOwner):
			h.writeError(w, http.StatusForbidden, "you are not the owner of this route", "")
		case errors.Is(err, service.ErrTitleTooShort):
			h.writeError(w, http.StatusBadRequest, "title must be at least 3 characters", "")
		case errors.Is(err, service.ErrInvalidWaypoints):
			h.writeError(w, http.StatusBadRequest, "route must have at least one waypoint", "")
		default:
			if strings.Contains(err.Error(), "transfers") {
				h.writeError(w, http.StatusBadRequest, "invalid transfers", err.Error())
				return
			}
			log.Error("Ошибка обновления маршрута", "error", err, "route_id", routeID)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	response := routeToResponse(route)
	h.writeJSON(w, http.StatusOK, response)
}

// GetPublicRoutes возвращает список публичных маршрутов
func (h *RouteHandler) GetPublicRoutes(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetPublicRoutes: запрос списка публичных маршрутов")

	// Парсим параметры запроса
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 10000 {
		size = 10000
	}
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "date"
	}

	// Фильтры
	filters := make(map[string]interface{})

	if country := r.URL.Query().Get("country"); country != "" {
		filters["country"] = country
	}

	if durationFrom := r.URL.Query().Get("duration_from"); durationFrom != "" {
		if val, err := strconv.Atoi(durationFrom); err == nil && val > 0 {
			filters["duration_from"] = val
		}
	}

	if durationTo := r.URL.Query().Get("duration_to"); durationTo != "" {
		if val, err := strconv.Atoi(durationTo); err == nil && val > 0 {
			filters["duration_to"] = val
		}
	}

	if tags := r.URL.Query().Get("tags"); tags != "" {
		tagList := strings.Split(tags, ",")
		for i, tag := range tagList {
			tagList[i] = strings.TrimSpace(tag)
		}
		filters["tags"] = tagList
	}

	userID := h.getUserIDFromContext(r)

	routes, total, err := h.routeService.ListPublicRoutes(r.Context(), page, size, sort, filters, userID)
	if err != nil {
		log.Error("Ошибка получения публичных маршрутов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]routedto.RouteResponse, len(routes))
	for i, route := range routes {
		items[i] = routedto.RouteResponse{
			ID:          route.ID,
			Title:       route.Title,
			Description: route.Description,
			Country:     route.Country,
			ImageURL:    route.ImageURL,
			UserID:      route.UserID,
			IsPublic:    route.IsPublic,
			TotalDays:   route.TotalDays,
			TotalHours:  route.TotalHours,
			LikesCount:  route.LikesCount,
			EventsCount: route.EventsCount,
			IsLiked:     route.IsLiked,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		}
	}

	totalPages := (int(total) + size - 1) / size

	response := routedto.ListRoutesResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetMyRoutes возвращает маршруты текущего пользователя
func (h *RouteHandler) GetMyRoutes(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetMyRoutes: запрос маршрутов пользователя")

	userID := h.getUserIDFromContext(r)
	if userID == nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 || size > 100 {
		size = 10
	}

	routes, total, err := h.routeService.ListUserRoutes(r.Context(), *userID, page, size)
	if err != nil {
		log.Error("Ошибка получения маршрутов пользователя", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]routedto.RouteResponse, len(routes))
	for i, route := range routes {
		items[i] = routedto.RouteResponse{
			ID:          route.ID,
			Title:       route.Title,
			Description: route.Description,
			Country:     route.Country,
			ImageURL:    route.ImageURL,
			UserID:      route.UserID,
			IsPublic:    route.IsPublic,
			TotalDays:   route.TotalDays,
			TotalHours:  route.TotalHours,
			LikesCount:  route.LikesCount,
			EventsCount: route.EventsCount,
			IsLiked:     route.IsLiked,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		}
	}

	totalPages := (int(total) + size - 1) / size

	response := routedto.ListRoutesResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetPopularRoutes возвращает популярные маршруты
func (h *RouteHandler) GetPopularRoutes(w http.ResponseWriter, r *http.Request) {
	log := h.log.WithRequestID(r.Context())
	log.Debug("GetPopularRoutes: запрос популярных маршрутов")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	userID := h.getUserIDFromContext(r)

	routes, err := h.routeService.GetPopularRoutes(r.Context(), limit, userID)
	if err != nil {
		log.Error("Ошибка получения популярных маршрутов", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	items := make([]routedto.RouteResponse, len(routes))
	for i, route := range routes {
		items[i] = routedto.RouteResponse{
			ID:          route.ID,
			Title:       route.Title,
			Description: route.Description,
			Country:     route.Country,
			ImageURL:    route.ImageURL,
			UserID:      route.UserID,
			IsPublic:    route.IsPublic,
			TotalDays:   route.TotalDays,
			TotalHours:  route.TotalHours,
			LikesCount:  route.LikesCount,
			EventsCount: route.EventsCount,
			IsLiked:     route.IsLiked,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		}
	}

	h.writeJSON(w, http.StatusOK, items)
}
