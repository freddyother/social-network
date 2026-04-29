package handlers

import (
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

func (h SocialHandler) HandlePost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	post, err := h.service.PostByID(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("postID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"post": post,
	})
}

func (h SocialHandler) HandleGroups(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	groups, err := h.service.Groups(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"groups": groups,
	})
}

func (h SocialHandler) HandleGroup(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	group, err := h.service.GroupByID(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"group": group,
	})
}

func (h SocialHandler) HandleCreateGroup(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	group, err := h.service.CreateGroup(r.Context(), *currentUser, social.CreateGroupInput{
		Title:       payload.Title,
		Description: payload.Description,
	})
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"group": group,
	})
}

func (h SocialHandler) HandleJoinGroup(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	group, err := h.service.JoinGroup(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("groupID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"group": group,
	})
}

func (h SocialHandler) HandleSearch(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	results, err := h.service.Search(r.Context(), currentUser.ID, r.URL.Query().Get("q"))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, results)
}
