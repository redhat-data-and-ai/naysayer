package app

import "github.com/redhat-data-and-ai/gitlab-bot-backend/library"

type MergeRequestEvent struct {
	ObjectKind string `json:"object_kind"`

	User struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"user"`

	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`

	ObjectAttributes map[string]any `json:"object_attributes"`
}

type Handler interface {
	Handle(event MergeRequestEvent, gitLabClient library.GitLabClient) error
}
