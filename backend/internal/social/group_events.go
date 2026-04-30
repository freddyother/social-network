package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"social-network/backend/internal/auth"
)

const (
	maxGroupEventTitleLength       = 160
	maxGroupEventDescriptionLength = 2000
)

type GroupEvent struct {
	ID             string    `json:"id"`
	GroupID        string    `json:"groupId"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	StartsAt       time.Time `json:"startsAt"`
	CreatedAt      time.Time `json:"createdAt"`
	GoingCount     int       `json:"goingCount"`
	NotGoingCount  int       `json:"notGoingCount"`
	ViewerResponse string    `json:"viewerResponse,omitempty"`
	Creator        GroupUser `json:"creator"`
}

type CreateGroupEventInput struct {
	Title       string
	Description string
	StartsAt    time.Time
}

func (s Service) GroupEvents(ctx context.Context, viewerID, groupID string) ([]GroupEvent, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, viewerID, groupID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				ge.id,
				ge.group_id,
				ge.title,
				ge.description,
				ge.starts_at,
				ge.created_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				viewer.response,
				COALESCE(response_counts.going_count, 0) AS going_count,
				COALESCE(response_counts.not_going_count, 0) AS not_going_count
			FROM group_events ge
			INNER JOIN users u ON u.id = ge.creator_id
			LEFT JOIN event_responses viewer
				ON viewer.event_id = ge.id
				AND viewer.user_id = $2
			LEFT JOIN (
				SELECT
					event_id,
					COUNT(*) FILTER (WHERE response = 'going')::INT AS going_count,
					COUNT(*) FILTER (WHERE response = 'not_going')::INT AS not_going_count
				FROM event_responses
				GROUP BY event_id
			) response_counts ON response_counts.event_id = ge.id
			WHERE ge.group_id = $1
			ORDER BY ge.starts_at ASC, ge.created_at DESC, ge.id DESC
		`,
		normalizedGroupID,
		viewerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group events: %w", err)
	}
	defer rows.Close()

	return s.loadGroupEventsFromRows(rows, "group events")
}

func (s Service) CreateGroupEvent(ctx context.Context, creator auth.User, groupID string, input CreateGroupEventInput) (GroupEvent, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, creator.ID, groupID)
	if err != nil {
		return GroupEvent{}, err
	}

	normalizedInput, err := normalizeCreateGroupEventInput(input)
	if err != nil {
		return GroupEvent{}, err
	}

	eventID, err := newToken(16)
	if err != nil {
		return GroupEvent{}, fmt.Errorf("generate group event id: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupEvent{}, fmt.Errorf("begin create group event transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO group_events (id, group_id, creator_id, title, description, starts_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
		eventID,
		normalizedGroupID,
		creator.ID,
		normalizedInput.Title,
		normalizedInput.Description,
		normalizedInput.StartsAt,
	); err != nil {
		return GroupEvent{}, fmt.Errorf("insert group event: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO event_responses (event_id, user_id, response)
			VALUES ($1, $2, 'going')
			ON CONFLICT (event_id, user_id) DO UPDATE SET
				response = 'going',
				responded_at = NOW()
		`,
		eventID,
		creator.ID,
	); err != nil {
		return GroupEvent{}, fmt.Errorf("mark group event creator as going: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return GroupEvent{}, fmt.Errorf("commit create group event transaction: %w", err)
	}

	return s.loadGroupEventByID(ctx, creator.ID, normalizedGroupID, eventID)
}

func (s Service) RespondToGroupEvent(ctx context.Context, userID, groupID, eventID, response string) (GroupEvent, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, userID, groupID)
	if err != nil {
		return GroupEvent{}, err
	}

	normalizedEventID := strings.TrimSpace(eventID)
	if normalizedEventID == "" {
		return GroupEvent{}, &ValidationError{
			Message: "Choose an event first.",
			Fields: map[string]string{
				"eventId": "Choose an event first.",
			},
		}
	}

	normalizedResponse := strings.ToLower(strings.TrimSpace(response))
	if normalizedResponse != "going" && normalizedResponse != "not_going" {
		return GroupEvent{}, &ValidationError{
			Message: "Choose a valid RSVP.",
			Fields: map[string]string{
				"response": "Choose either going or not going.",
			},
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupEvent{}, fmt.Errorf("begin group event response transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var exists bool
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM group_events
				WHERE id = $1 AND group_id = $2
			)
		`,
		normalizedEventID,
		normalizedGroupID,
	).Scan(&exists); err != nil {
		return GroupEvent{}, fmt.Errorf("check group event existence: %w", err)
	}
	if !exists {
		return GroupEvent{}, ErrNotFound
	}

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO event_responses (event_id, user_id, response)
			VALUES ($1, $2, $3)
			ON CONFLICT (event_id, user_id)
			DO UPDATE SET
				response = EXCLUDED.response,
				responded_at = NOW()
		`,
		normalizedEventID,
		userID,
		normalizedResponse,
	); err != nil {
		return GroupEvent{}, fmt.Errorf("upsert group event response: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return GroupEvent{}, fmt.Errorf("commit group event response transaction: %w", err)
	}

	return s.loadGroupEventByID(ctx, userID, normalizedGroupID, normalizedEventID)
}

func (s Service) loadGroupEventByID(ctx context.Context, viewerID, groupID, eventID string) (GroupEvent, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				ge.id,
				ge.group_id,
				ge.title,
				ge.description,
				ge.starts_at,
				ge.created_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				viewer.response,
				COALESCE(response_counts.going_count, 0) AS going_count,
				COALESCE(response_counts.not_going_count, 0) AS not_going_count
			FROM group_events ge
			INNER JOIN users u ON u.id = ge.creator_id
			LEFT JOIN event_responses viewer
				ON viewer.event_id = ge.id
				AND viewer.user_id = $3
			LEFT JOIN (
				SELECT
					event_id,
					COUNT(*) FILTER (WHERE response = 'going')::INT AS going_count,
					COUNT(*) FILTER (WHERE response = 'not_going')::INT AS not_going_count
				FROM event_responses
				GROUP BY event_id
			) response_counts ON response_counts.event_id = ge.id
			WHERE ge.group_id = $1 AND ge.id = $2
			LIMIT 1
		`,
		groupID,
		eventID,
		viewerID,
	)
	if err != nil {
		return GroupEvent{}, fmt.Errorf("query group event by id: %w", err)
	}
	defer rows.Close()

	events, err := s.loadGroupEventsFromRows(rows, "group event")
	if err != nil {
		return GroupEvent{}, err
	}
	if len(events) == 0 {
		return GroupEvent{}, ErrNotFound
	}

	return events[0], nil
}

func (s Service) loadGroupEventsFromRows(rows *sql.Rows, operation string) ([]GroupEvent, error) {
	events := make([]GroupEvent, 0)

	for rows.Next() {
		var (
			event           GroupEvent
			creatorNickname sql.NullString
			creatorAvatar   sql.NullString
			viewerResponse  sql.NullString
		)
		if err := rows.Scan(
			&event.ID,
			&event.GroupID,
			&event.Title,
			&event.Description,
			&event.StartsAt,
			&event.CreatedAt,
			&event.Creator.ID,
			&event.Creator.FirstName,
			&event.Creator.LastName,
			&creatorNickname,
			&creatorAvatar,
			&viewerResponse,
			&event.GoingCount,
			&event.NotGoingCount,
		); err != nil {
			return nil, fmt.Errorf("scan %s group event: %w", operation, err)
		}

		event.Creator.Nickname = nullStringValue(creatorNickname)
		if creatorAvatar.Valid {
			event.Creator.AvatarURL = s.publicURL(creatorAvatar.String)
		}
		event.ViewerResponse = nullStringValue(viewerResponse)

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s group events: %w", operation, err)
	}

	return events, nil
}

func normalizeCreateGroupEventInput(input CreateGroupEventInput) (CreateGroupEventInput, error) {
	normalized := CreateGroupEventInput{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		StartsAt:    input.StartsAt.UTC(),
	}

	fieldErrors := make(map[string]string)
	if normalized.Title == "" {
		fieldErrors["title"] = "Event title is required."
	} else if len(normalized.Title) > maxGroupEventTitleLength {
		fieldErrors["title"] = "Event title must be 160 characters or fewer."
	}

	if normalized.Description == "" {
		fieldErrors["description"] = "Event description is required."
	} else if len(normalized.Description) > maxGroupEventDescriptionLength {
		fieldErrors["description"] = "Event description must be 2000 characters or fewer."
	}

	if input.StartsAt.IsZero() {
		fieldErrors["startsAt"] = "Choose when the event starts."
	}

	if len(fieldErrors) > 0 {
		return CreateGroupEventInput{}, &ValidationError{
			Message: "Please correct the event details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}
