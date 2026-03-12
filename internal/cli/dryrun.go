package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

const (
	planningModeStatic   = "static"
	planningModeStateful = "stateful"

	capabilityFull    = "full"
	capabilityPartial = "partial"
)

type dryRunProfile struct {
	Intent        string
	Action        string
	PlanningMode  string
	Capability    string
	CapabilityMsg string
}

type dryRunItem struct {
	Intent          string         `json:"intent"`
	Target          map[string]any `json:"target"`
	Action          string         `json:"action"`
	PredictedAction string         `json:"predicted_action,omitempty"`
	Supported       bool           `json:"supported"`
	Reason          string         `json:"reason,omitempty"`
	Confidence      string         `json:"confidence,omitempty"`
	RequiredState   []string       `json:"required_state,omitempty"`
	BlockingReasons []string       `json:"blocking_reasons,omitempty"`
}

type dryRunSummary struct {
	Total       int `json:"total"`
	Supported   int `json:"supported"`
	Unsupported int `json:"unsupported"`

	NoopCount    int `json:"no_op"`
	CreateCount  int `json:"create"`
	UpdateCount  int `json:"update"`
	DeleteCount  int `json:"delete"`
	UnknownCount int `json:"unknown"`
}

type dryRunPreview struct {
	DryRun       bool          `json:"dry_run"`
	PlanningMode string        `json:"planning_mode"`
	Capability   string        `json:"capability"`
	Items        []dryRunItem  `json:"items"`
	Summary      dryRunSummary `json:"summary"`
}

var dryRunProfiles = map[string]dryRunProfile{
	"branch create":                                {Intent: "branch.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"branch default set":                           {Intent: "branch.default.set", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"branch model update":                          {Intent: "branch.model.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"branch restriction create":                    {Intent: "branch.restriction.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"branch restriction update":                    {Intent: "branch.restriction.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"branch restriction delete":                    {Intent: "branch.restriction.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"build status set":                             {Intent: "build.status.set", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"build required create":                        {Intent: "build.required.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"build required update":                        {Intent: "build.required.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"build required delete":                        {Intent: "build.required.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"tag create":                                   {Intent: "tag.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"tag delete":                                   {Intent: "tag.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo comment create":                          {Intent: "repo.comment.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo comment update":                          {Intent: "repo.comment.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo comment delete":                          {Intent: "repo.comment.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo settings workflow webhooks create":       {Intent: "repo.webhook.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo settings workflow webhooks delete":       {Intent: "repo.webhook.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo settings pull-requests update":           {Intent: "repo.pull-request-settings.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo settings pull-requests update-approvers": {Intent: "repo.pull-request-settings.update-approvers", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo settings pull-requests set-strategy":     {Intent: "repo.pull-request-settings.set-strategy", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo admin create":                            {Intent: "repo.admin.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo admin fork":                              {Intent: "repo.admin.fork", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo admin update":                            {Intent: "repo.admin.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"repo admin delete":                            {Intent: "repo.admin.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"insights report set":                          {Intent: "insights.report.set", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"insights report delete":                       {Intent: "insights.report.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"insights annotation add":                      {Intent: "insights.annotation.add", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"insights annotation delete":                   {Intent: "insights.annotation.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr create":                                    {Intent: "pr.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr update":                                    {Intent: "pr.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr merge":                                     {Intent: "pr.merge", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr decline":                                   {Intent: "pr.decline", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr reopen":                                    {Intent: "pr.reopen", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr review approve":                            {Intent: "pr.review.approve", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr review unapprove":                          {Intent: "pr.review.unapprove", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr review reviewer add":                       {Intent: "pr.review.reviewer.add", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr review reviewer remove":                    {Intent: "pr.review.reviewer.remove", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr task create":                               {Intent: "pr.task.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr task update":                               {Intent: "pr.task.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"pr task delete":                               {Intent: "pr.task.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"reviewer condition create":                    {Intent: "reviewer.condition.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"reviewer condition update":                    {Intent: "reviewer.condition.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"reviewer condition delete":                    {Intent: "reviewer.condition.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"hook enable":                                  {Intent: "hook.enable", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"hook disable":                                 {Intent: "hook.disable", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"hook configure":                               {Intent: "hook.configure", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project create":                               {Intent: "project.create", Action: "create", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project update":                               {Intent: "project.update", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project delete":                               {Intent: "project.delete", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project permissions users grant":              {Intent: "project.permission.user.grant", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project permissions users revoke":             {Intent: "project.permission.user.revoke", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project permissions groups grant":             {Intent: "project.permission.group.grant", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"project permissions groups revoke":            {Intent: "project.permission.group.revoke", Action: "delete", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
	"bulk apply":                                   {Intent: "bulk.apply", Action: "update", PlanningMode: planningModeStatic, Capability: capabilityPartial, CapabilityMsg: "static preview only"},
}

var dryRunPassthroughPaths = map[string]struct{}{
	"branch delete": {},
	"repo settings security permissions users grant":   {},
	"repo settings security permissions users revoke":  {},
	"repo settings security permissions groups grant":  {},
	"repo settings security permissions groups revoke": {},
	"project permissions users grant":                  {},
	"project permissions users revoke":                 {},
	"project permissions groups grant":                 {},
	"project permissions groups revoke":                {},
	"hook enable":                                      {},
	"hook disable":                                     {},
	"hook configure":                                   {},
	"repo settings workflow webhooks create":           {},
	"repo settings workflow webhooks delete":           {},
	"repo settings pull-requests update":               {},
	"repo settings pull-requests update-approvers":     {},
	"repo settings pull-requests set-strategy":         {},
	"branch create":                                    {},
	"branch default set":                               {},
	"branch model update":                              {},
	"branch restriction create":                        {},
	"branch restriction update":                        {},
	"branch restriction delete":                        {},
	"tag create":                                       {},
	"tag delete":                                       {},
	"reviewer condition create":                        {},
	"reviewer condition update":                        {},
	"reviewer condition delete":                        {},
	"repo admin create":                                {},
	"repo admin fork":                                  {},
	"repo admin update":                                {},
	"repo admin delete":                                {},
	"project create":                                   {},
	"project update":                                   {},
	"project delete":                                   {},
	"pr create":                                        {},
	"pr update":                                        {},
	"pr merge":                                         {},
	"pr decline":                                       {},
	"pr reopen":                                        {},
	"pr review approve":                                {},
	"pr review unapprove":                              {},
	"pr review reviewer add":                           {},
	"pr review reviewer remove":                        {},
	"pr task create":                                   {},
	"pr task update":                                   {},
	"pr task delete":                                   {},
	"build status set":                                 {},
	"build required create":                            {},
	"build required update":                            {},
	"build required delete":                            {},
	"insights report set":                              {},
	"insights report delete":                           {},
	"insights annotation add":                          {},
	"insights annotation delete":                       {},
	"repo comment create":                              {},
	"repo comment update":                              {},
	"repo comment delete":                              {},
}

func registerGlobalDryRunInterceptors(root *cobra.Command, options *rootOptions) {
	if root == nil || options == nil {
		return
	}

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command == nil {
			return
		}

		path := dryRunCommandPath(command)
		profile, hasDryRunProfile := dryRunProfiles[path]
		if hasDryRunProfile && command.RunE != nil {
			originalRun := command.RunE
			command.RunE = func(cmd *cobra.Command, args []string) error {
				if !options.DryRun {
					return originalRun(cmd, args)
				}

				path := dryRunCommandPath(cmd)
				if _, ok := dryRunPassthroughPaths[path]; ok {
					return originalRun(cmd, args)
				}

				preview := newDryRunPreview(profile, cmd, args)
				return writeDryRunPreview(cmd.OutOrStdout(), options.JSON, preview)
			}
		} else if command.RunE != nil {
			originalRun := command.RunE
			command.RunE = func(cmd *cobra.Command, args []string) error {
				if !options.DryRun {
					return originalRun(cmd, args)
				}

				path := dryRunCommandPath(cmd)
				if _, ok := dryRunPassthroughPaths[path]; ok {
					return originalRun(cmd, args)
				}
				if !isServerMutatingPath(path) {
					return originalRun(cmd, args)
				}

				return apperrors.New(apperrors.KindNotImplemented, fmt.Sprintf("dry-run is not implemented for %s", path), nil)
			}
		}

		for _, child := range command.Commands() {
			visit(child)
		}
	}

	visit(root)
}

func isServerMutatingPath(path string) bool {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return false
	}

	parts := strings.Fields(trimmedPath)
	if len(parts) == 0 {
		return false
	}

	last := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	switch last {
	case "create", "update", "delete", "set", "grant", "revoke", "enable", "disable", "configure", "merge", "decline", "reopen", "approve", "unapprove", "add", "remove", "fork", "apply":
		return true
	default:
		return false
	}
}

func dryRunCommandPath(command *cobra.Command) string {
	if command == nil {
		return ""
	}

	path := strings.TrimSpace(command.CommandPath())
	path = strings.TrimPrefix(path, "bbsc ")
	return strings.TrimSpace(path)
}

func newDryRunPreview(profile dryRunProfile, command *cobra.Command, args []string) dryRunPreview {
	target := map[string]any{}
	repository := ""
	if command != nil {
		if flag := command.Flags().Lookup("repo"); flag != nil {
			repository = strings.TrimSpace(flag.Value.String())
		}
	}
	if repository != "" {
		target["repository"] = repository
	}
	if len(args) > 0 {
		target["args"] = append([]string(nil), args...)
	}

	item := dryRunItem{
		Intent:          profile.Intent,
		Target:          target,
		Action:          profile.Action,
		PredictedAction: profile.Action,
		Supported:       true,
		Reason:          strings.TrimSpace(profile.CapabilityMsg),
		Confidence:      capabilityPartial,
	}

	summary := dryRunSummary{Total: 1, Supported: 1}
	switch profile.Action {
	case "no-op":
		summary.NoopCount = 1
	case "create":
		summary.CreateCount = 1
	case "update":
		summary.UpdateCount = 1
	case "delete":
		summary.DeleteCount = 1
	default:
		summary.UnknownCount = 1
	}

	return dryRunPreview{
		DryRun:       true,
		PlanningMode: profile.PlanningMode,
		Capability:   profile.Capability,
		Items:        []dryRunItem{item},
		Summary:      summary,
	}
}

func writeDryRunPreview(writer io.Writer, asJSON bool, preview dryRunPreview) error {
	if asJSON {
		return writeJSON(writer, preview)
	}

	if _, err := fmt.Fprintf(writer, "Dry-run (%s, capability=%s)\n", preview.PlanningMode, preview.Capability); err != nil {
		return err
	}

	for _, item := range preview.Items {
		if _, err := fmt.Fprintf(writer, "- intent=%s action=%s", item.Intent, item.Action); err != nil {
			return err
		}
		if item.PredictedAction != "" {
			if _, err := fmt.Fprintf(writer, " predicted_action=%s", item.PredictedAction); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
		if repository, ok := item.Target["repository"].(string); ok && strings.TrimSpace(repository) != "" {
			if _, err := fmt.Fprintf(writer, "  repository=%s\n", repository); err != nil {
				return err
			}
		}
		if args, ok := item.Target["args"].([]string); ok && len(args) > 0 {
			if _, err := fmt.Fprintf(writer, "  args=%s\n", strings.Join(args, " ")); err != nil {
				return err
			}
		}
		if item.Reason != "" {
			if _, err := fmt.Fprintf(writer, "  note=%s\n", item.Reason); err != nil {
				return err
			}
		}
	}

	return nil
}
