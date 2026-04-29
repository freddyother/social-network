package handlers

import (
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

func (h SocialHandler) HandleGroupPosts(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	posts, err := h.service.GroupPosts(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"posts": posts,
	})
}

func (h SocialHandler) HandleCreateGroupPost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
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

	post, err := h.service.CreateGroupPost(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("groupID")),
		social.CreateGroupPostInput{
			Body: payload.Body,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"post": post,
	})
}
