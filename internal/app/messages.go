package app

import (
	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/localstore"
	"github.com/Ret2Hell/jiratui/internal/service"
)

type cacheLoadedMsg struct {
	State localstore.State
	OK    bool
	Err   error
}

type cacheSavedMsg struct{}

type sprintLoadedMsg struct {
	Data service.SprintData
}

type taskCreatedMsg struct {
	TempKey string
	Issue   jira.Issue
}

type taskCreateFailedMsg struct {
	TempKey string
	Err     error
}

type issueTransitionedMsg struct {
	Key    string
	Status jira.Status
}

type issueTransitionFailedMsg struct {
	Key    string
	Target jira.Status
	Err    error
}

type pointsUpdatedMsg struct {
	Key    string
	Points *float64
}

type pointsUpdateFailedMsg struct {
	Key    string
	Points *float64
	Err    error
}

type reportGeneratedMsg struct {
	Draft   service.DailyDraft
	Open    bool
	Version int
}

type draftSavedMsg struct{}

type jiraSetupSavedMsg struct {
	Config config.Config
}

type setupSavedMsg struct {
	Config  config.Config
	Service service.Service
}

type errMsg struct {
	Err error
}
