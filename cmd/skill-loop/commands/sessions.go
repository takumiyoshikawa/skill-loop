package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/takumiyoshikawa/skill-loop/internal/session"
)

func NewSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage tmux-backed run sessions",
	}

	cmd.AddCommand(newSessionsLsCmd())
	cmd.AddCommand(newSessionsAttachCmd())
	cmd.AddCommand(newSessionsStopCmd())

	return cmd
}

func newSessionsLsCmd() *cobra.Command {
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List recorded run sessions in the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := session.ResolveRepoRoot("")
			if err != nil {
				return err
			}

			metas, total, err := listRunSessions(repoRoot, offset, limit)
			if err != nil {
				return err
			}

			if len(metas) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tSTARTED\tLAST_OUTPUT")
			for _, meta := range metas {
				_ = session.Reconcile(meta)
				fmt.Fprintf(
					w,
					"%s\t%s\t%s\t%s\n",
					meta.ID,
					meta.Status,
					meta.StartedAt.Format(time.RFC3339),
					meta.LastOutputAt.Format(time.RFC3339),
				)
			}
			_ = w.Flush()

			fmt.Fprintf(os.Stderr, "\nShowing %d-%d of %d sessions\n", offset+1, offset+len(metas), total)
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of sessions to display")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of sessions to skip")

	return cmd
}

func newSessionsAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach to a run session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadRunSessionByID(args[0])
			if err != nil {
				return err
			}
			return session.Attach(meta)
		},
	}
}

func newSessionsStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <session-id>",
		Short: "Stop a run session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := loadRunSessionByID(args[0])
			if err != nil {
				return err
			}
			return session.Stop(meta)
		},
	}
}

func loadRunSessionByID(id string) (*session.Metadata, error) {
	repoRoot, err := session.ResolveRepoRoot("")
	if err != nil {
		return nil, err
	}
	meta, err := session.LoadByID(repoRoot, id)
	if err != nil {
		return nil, fmt.Errorf("load session %s: %w (expected at %s)", id, err, filepath.Join(session.SessionsRoot(repoRoot), id, "session.json"))
	}
	if meta.Skill != "orchestrator" {
		return nil, fmt.Errorf("session %s is not a run session", id)
	}
	return meta, nil
}

func listRunSessions(repoRoot string, offset, limit int) ([]*session.Metadata, int, error) {
	all, err := session.List(repoRoot)
	if err != nil {
		return nil, 0, err
	}

	runs := make([]*session.Metadata, 0, len(all))
	for _, meta := range all {
		if meta.Skill == "orchestrator" {
			runs = append(runs, meta)
		}
	}

	total := len(runs)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*session.Metadata{}, total, nil
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	return runs[offset:end], total, nil
}
