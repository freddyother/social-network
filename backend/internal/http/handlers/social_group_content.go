package handlers

import (
	stdlibhttp "net/http"
	"strings"
	"time"

	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

func (h SocialHandler) HandleGroupComments(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	comments, err := h.service.GroupComments(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("postID")),
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"comments": comments,
	})
}

func (h SocialHandler) HandleCreateGroupComment(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	comment, err := h.service.CreateGroupComment(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("postID")),
		social.CreateGroupCommentInput{
			Body: payload.Body,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"comment": comment,
	})
}

func (h SocialHandler) HandleGroupEvents(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	events, err := h.service.GroupEvents(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"events": events,
	})
}

func (h SocialHandler) HandleCreateGroupEvent(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartsAt    time.Time `json:"startsAt"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	event, err := h.service.CreateGroupEvent(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("groupID")),
		social.CreateGroupEventInput{
			Title:       payload.Title,
			Description: payload.Description,
			StartsAt:    payload.StartsAt,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"event": event,
	})
}

func (h SocialHandler) HandleRespondGroupEvent(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Response string `json:"response"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	event, err := h.service.RespondToGroupEvent(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("eventID")),
		payload.Response,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"event": event,
	})
}

func (h SocialHandler) HandleInviteUserToGroup(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		RecipientID string `json:"recipientId"`
		Note        string `json:"note"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	message, err := h.service.InviteUserToGroup(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("groupID")),
		social.InviteUserToGroupInput{
			RecipientID: payload.RecipientID,
			Note:        payload.Note,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"message": message,
	})
}
