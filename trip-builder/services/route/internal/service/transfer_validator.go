
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/route/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

var (
	ErrInvalidTransferOrder    = errors.New("invalid transfer order: from_order must be less than to_order")
	ErrDuplicateTransfer       = errors.New("duplicate transfer between waypoints")
	ErrTransferToSameWaypoint  = errors.New("transfer cannot go to the same waypoint")
	ErrTransferOrderGap        = errors.New("transfer order must be consecutive or circular")
	ErrWaypointNotFound        = errors.New("waypoint not found for transfer")
)

type TransferValidator struct {
	log *logger.Logger
}

func NewTransferValidator(log *logger.Logger) *TransferValidator {
	return &TransferValidator{log: log}
}

// ValidateForCreate валидирует перемещения при создании маршрута
func (v *TransferValidator) ValidateForCreate(
	ctx context.Context,
	waypoints []models.CreateWaypoint,
	transfers []models.CreateTransferRequest,
) error {
	v.log.Debug("TransferValidator.ValidateForCreate: начало",
		"waypoints_count", len(waypoints),
		"transfers_count", len(transfers),
	)

	if len(transfers) == 0 {
		v.log.Debug("TransferValidator.ValidateForCreate: нет перемещений, валидация не требуется")
		return nil
	}

	// Проверяем, что количество перемещений допустимо
	maxTransfers := len(waypoints)
	if len(transfers) > maxTransfers {
		v.log.Warn("TransferValidator.ValidateForCreate: слишком много перемещений",
			"max_allowed", maxTransfers,
			"got", len(transfers),
		)
		return fmt.Errorf("transfers count (%d) exceeds maximum allowed (%d)", len(transfers), maxTransfers)
	}

	// Собираем мапу использованных переходов для проверки дублей
	usedTransitions := make(map[string]bool)

	// Проверяем каждое перемещение
	for i, t := range transfers {
		// Проверка, что from и to разные
		if t.FromOrderIndex == t.ToOrderIndex {
			v.log.Warn("TransferValidator.ValidateForCreate: перемещение в ту же точку",
				"index", i,
				"order", t.FromOrderIndex,
			)
			return fmt.Errorf("%w at index %d", ErrTransferToSameWaypoint, i)
		}

		// Проверка порядка
		if t.FromOrderIndex >= t.ToOrderIndex {
			// Разрешено только если это замыкание круга (последняя -> первая)
			isCircular := t.FromOrderIndex == len(waypoints)-1 && t.ToOrderIndex == 0
			if !isCircular {
				v.log.Warn("TransferValidator.ValidateForCreate: неверный порядок перемещения",
					"index", i,
					"from", t.FromOrderIndex,
					"to", t.ToOrderIndex,
				)
				return fmt.Errorf("%w at index %d: from=%d, to=%d",
					ErrInvalidTransferOrder, i, t.FromOrderIndex, t.ToOrderIndex)
			}
		}

		// Проверка дублей
		transitionKey := fmt.Sprintf("%d->%d", t.FromOrderIndex, t.ToOrderIndex)
		if usedTransitions[transitionKey] {
			v.log.Warn("TransferValidator.ValidateForCreate: дублирующееся перемещение",
				"transition", transitionKey,
			)
			return fmt.Errorf("%w: %s", ErrDuplicateTransfer, transitionKey)
		}
		usedTransitions[transitionKey] = true

		// Проверка валидности длительности
		if t.DurationHours == 0 && t.DurationMinutes == 0 {
			v.log.Warn("TransferValidator.ValidateForCreate: нулевая длительность перемещения",
				"index", i,
			)
		}

		// Проверка минут
		if t.DurationMinutes < 0 || t.DurationMinutes >= 60 {
			return fmt.Errorf("invalid duration_minutes at index %d: must be between 0 and 59", i)
		}
	}

	// Проверяем, что все перемещения образуют связный путь
	if err := v.validateConnectivity(waypoints, transfers); err != nil {
		return err
	}

	v.log.Info("TransferValidator.ValidateForCreate: валидация пройдена")

	return nil
}

// ValidateForUpdate валидирует перемещения при обновлении маршрута
func (v *TransferValidator) ValidateForUpdate(
	ctx context.Context,
	waypoints []models.Waypoint,
	transfers []models.UpdateTransferRequest,
) error {
	v.log.Debug("TransferValidator.ValidateForUpdate: начало",
		"waypoints_count", len(waypoints),
		"transfers_count", len(transfers),
	)

	if len(transfers) == 0 {
		return nil
	}

	// Конвертируем в формат для валидации
	createTransfers := make([]models.CreateTransferRequest, len(transfers))
	for i, t := range transfers {
		fromOrder := 0
		if t.FromOrderIndex != nil {
			fromOrder = *t.FromOrderIndex
		}
		toOrder := 0
		if t.ToOrderIndex != nil {
			toOrder = *t.ToOrderIndex
		}
		durationHours := 0
		if t.DurationHours != nil {
			durationHours = *t.DurationHours
		}
		durationMinutes := 0
		if t.DurationMinutes != nil {
			durationMinutes = *t.DurationMinutes
		}
		transportType := models.TransportCar
		if t.TransportType != nil {
			transportType = *t.TransportType
		}

		createTransfers[i] = models.CreateTransferRequest{
			FromOrderIndex:  fromOrder,
			ToOrderIndex:    toOrder,
			DurationHours:   durationHours,
			DurationMinutes: durationMinutes,
			TransportType:   transportType,
		}
	}

	// Конвертируем waypoints в формат для валидации
	createWaypoints := make([]models.CreateWaypoint, len(waypoints))
	for i, wp := range waypoints {
		createWaypoints[i] = models.CreateWaypoint{
			City:          wp.City,
			DurationDays:  wp.DurationDays,
			DurationHours: wp.DurationHours,
			OrderIndex:    wp.OrderIndex,
		}
	}

	return v.ValidateForCreate(ctx, createWaypoints, createTransfers)
}

// validateConnectivity проверяет, что перемещения образуют связный путь
func (v *TransferValidator) validateConnectivity(
	waypoints []models.CreateWaypoint,
	transfers []models.CreateTransferRequest,
) error {
	if len(transfers) == 0 {
		return nil
	}

	// Строим граф переходов
	graph := make(map[int][]int)
	for _, t := range transfers {
		graph[t.FromOrderIndex] = append(graph[t.FromOrderIndex], t.ToOrderIndex)
	}

	// Проверяем, что из каждой точки (кроме последней) есть переход
	for i := 0; i < len(waypoints)-1; i++ {
		if _, ok := graph[i]; !ok {
			v.log.Warn("TransferValidator.validateConnectivity: нет перехода из точки",
				"order_index", i,
			)
			return fmt.Errorf("%w: missing transfer from waypoint at order %d", ErrTransferOrderGap, i)
		}
	}

	// Проверяем, есть ли переход из последней точки (кольцевой маршрут)
	hasCircular := false
	if dests, ok := graph[len(waypoints)-1]; ok {
		for _, dest := range dests {
			if dest == 0 {
				hasCircular = true
				break
			}
		}
	}

	v.log.Debug("TransferValidator.validateConnectivity: результат",
		"waypoints_count", len(waypoints),
		"has_circular", hasCircular,
	)

	return nil
}