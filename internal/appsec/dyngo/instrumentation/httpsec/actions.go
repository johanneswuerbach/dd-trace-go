// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2022 Datadog, Inc.

package httpsec

import (
	"net/http"

	"gopkg.in/DataDog/dd-trace-go.v1/internal/log"
)

// Action is used to identify any action kind
type Action interface {
	isAction()
}

// BlockRequestAction is the action that holds the HTTP handler to use to block the request
type BlockRequestAction struct {
	// handler is the http handler to use to block the request
	handler http.Handler
}

func (*BlockRequestAction) isAction() {}

func NewBlockRequestAction(status int, template string) BlockRequestAction {
	htmlHandler := newBlockRequestHandler(status, "application/html", BlockedTemplateHTML)
	jsonHandler := newBlockRequestHandler(status, "application/json", BlockedTemplateJSON)
	var action BlockRequestAction
	switch template {
	case "json":
		action.handler = newBlockRequestHandler(status, "application/json", BlockedTemplateJSON)
		break
	case "html":
		action.handler = newBlockRequestHandler(status, "application/html", BlockedTemplateHTML)
		break
	default:
		action.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := jsonHandler
			for _, value := range r.Header.Values("Accept") {
				if value == "application/html" {
					h = htmlHandler
					break
				}
			}
			h.ServeHTTP(w, r)
		})
		break
	}
	return action

}

func newBlockRequestHandler(status int, ct string, payload []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(status)
		w.Write(payload)
	})
}

// ActionsHandler handles actions registration and their application to operations
type ActionsHandler struct {
	actions map[string]Action
}

// NewActionsHandler returns an action handler holding the default ASM actions.
// Currently, only the default "block" action is supported
func NewActionsHandler() *ActionsHandler {
	handler := ActionsHandler{
		actions: map[string]Action{},
	}
	// Register the default "block" action as specified in the RFC for HTTP blocking
	block := NewBlockRequestAction(403, "auto")
	handler.RegisterAction("block", &block)

	return &handler
}

// RegisterAction registers a specific action to the handler. If the action kind is unknown
// the action will not be registered
func (h *ActionsHandler) RegisterAction(id string, a Action) {
	h.actions[id] = a
}

// Apply applies the action identified by `id` for the given operation
// Returns true if the applied action will interrupt the request flow (block, redirect, etc...)
func (h *ActionsHandler) Apply(id string, op *Operation) bool {
	a, ok := h.actions[id]
	if !ok {
		log.Debug("appsec: ignoring the returned waf action: unknown action id `%s`", id)
		return false
	}
	op.AddAction(a)

	switch a.(type) {
	case *BlockRequestAction:
		return true
	default:
		return false
	}
}
