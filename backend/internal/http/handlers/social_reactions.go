package handlers

import (
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/http/response"
)

func (h SocialHandler) HandleSetPostReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reactionType, ok := h.decodeReactionPayload(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.SetPostReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("postID")),
		reactionType,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleClearPostReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.ClearPostReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("postID")),
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleSetCommentReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reactionType, ok := h.decodeReactionPayload(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.SetCommentReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("postID")),
		strings.TrimSpace(r.PathValue("commentID")),
		reactionType,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleClearCommentReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.ClearCommentReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("postID")),
		strings.TrimSpace(r.PathValue("commentID")),
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleSetGroupPostReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reactionType, ok := h.decodeReactionPayload(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.SetGroupPostReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("postID")),
		reactionType,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleClearGroupPostReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.ClearGroupPostReaction(
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
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleSetGroupCommentReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reactionType, ok := h.decodeReactionPayload(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.SetGroupCommentReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("postID")),
		strings.TrimSpace(r.PathValue("commentID")),
		reactionType,
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) HandleClearGroupCommentReaction(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	reaction, err := h.service.ClearGroupCommentReaction(
		r.Context(),
		currentUser.ID,
		strings.TrimSpace(r.PathValue("groupID")),
		strings.TrimSpace(r.PathValue("postID")),
		strings.TrimSpace(r.PathValue("commentID")),
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"reaction": reaction,
	})
}

func (h SocialHandler) decodeReactionPayload(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) (string, bool) {
	var payload struct {
		Reaction string `json:"reaction"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return "", false
	}

	return payload.Reaction, true
}
