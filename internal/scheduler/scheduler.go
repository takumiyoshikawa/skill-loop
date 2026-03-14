package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/takumiyoshikawa/skill-loop/internal/config"
	"github.com/takumiyoshikawa/skill-loop/internal/orchestrator"
	"github.com/takumiyoshikawa/skill-loop/internal/session"
)

type progressObserver struct {
	onIteration     func(iteration int, maxIterations int, skill string)
	onSkillComplete func(iteration int, maxIterations int, skill string, stdout string)
}

func (p *progressObserver) IterationStarted(iteration int, maxIterations int, skill string) {
	if p.onIteration != nil {
		p.onIteration(iteration, maxIterations, skill)
	}
}

func (p *progressObserver) SkillCompleted(iteration int, maxIterations int, skill string, stdout string) {
	if p.onSkillComplete != nil {
		p.onSkillComplete(iteration, maxIterations, skill, stdout)
	}
}

func Run(repoRoot string, sessionID string, cfg *config.Config, maxIterations int, prompt string, entrypoint string) error {
	if cfg.Schedule == "" {
		return fmt.Errorf("schedule is required for scheduler mode")
	}

	schedule, err := cron.ParseStandard(cfg.Schedule)
	if err != nil {
		return fmt.Errorf("parse schedule: %w", err)
	}

	if maxIterations <= 0 {
		maxIterations = cfg.MaxIterations
	}
	if maxIterations <= 0 {
		maxIterations = orchestrator.DefaultMaxIterations
	}

	if entrypoint == "" {
		entrypoint = cfg.DefaultEntrypoint
	}

	updateMeta := func(update func(*session.Metadata)) error {
		meta, err := session.LoadByID(repoRoot, sessionID)
		if err != nil {
			return err
		}
		update(meta)
		return session.Save(meta)
	}

	updateScheduled := func(lastErr string) error {
		nextRun := schedule.Next(time.Now())
		return updateMeta(func(meta *session.Metadata) {
			meta.Schedule = cfg.Schedule
			meta.Status = session.StatusScheduled
			meta.NextRun = &nextRun
			meta.CurrentIteration = 0
			meta.MaxIterations = maxIterations
			meta.CurrentSkill = ""
			meta.BlockReason = ""
			meta.ResumeSkill = ""
			meta.ResumePrompt = ""
			meta.LastError = lastErr
			meta.EndedAt = nil
		})
	}

	if err := updateScheduled(""); err != nil {
		return err
	}

	c := cron.New()
	var runMu sync.Mutex
	running := false

	_, err = c.AddFunc(cfg.Schedule, func() {
		meta, loadErr := session.LoadByID(repoRoot, sessionID)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "scheduled run skipped: failed to load session metadata: %v\n", loadErr)
			return
		}
		if meta.Status == session.StatusBlocked {
			fmt.Fprintln(os.Stderr, "scheduled run skipped: session is blocked awaiting human input")
			return
		}

		runMu.Lock()
		if running {
			runMu.Unlock()
			fmt.Fprintln(os.Stderr, "scheduled run skipped: previous execution still in progress")
			return
		}
		running = true
		runMu.Unlock()

		defer func() {
			runMu.Lock()
			running = false
			runMu.Unlock()
		}()

		if err := updateMeta(func(meta *session.Metadata) {
			meta.Status = session.StatusRunning
			meta.NextRun = nil
			meta.CurrentIteration = 0
			meta.MaxIterations = maxIterations
			meta.CurrentSkill = ""
			meta.BlockReason = ""
			meta.ResumeSkill = ""
			meta.ResumePrompt = ""
			meta.LastError = ""
			meta.EndedAt = nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "failed to persist running session state: %v\n", err)
		}

		observer := &progressObserver{
			onIteration: func(iteration int, maxIters int, skill string) {
				if err := updateMeta(func(meta *session.Metadata) {
					meta.Status = session.StatusRunning
					meta.CurrentIteration = iteration
					meta.MaxIterations = maxIters
					meta.CurrentSkill = skill
				}); err != nil {
					fmt.Fprintf(os.Stderr, "failed to persist session progress: %v\n", err)
				}
			},
			onSkillComplete: func(iteration int, maxIters int, skill string, stdout string) {
				if err := updateMeta(func(meta *session.Metadata) {
					meta.Status = session.StatusRunning
					meta.CurrentIteration = iteration
					meta.MaxIterations = maxIters
					meta.CurrentSkill = skill
					meta.LastSkillOutput = stdout
				}); err != nil {
					fmt.Fprintf(os.Stderr, "failed to persist session output: %v\n", err)
				}
			},
		}

		runErr := orchestrator.RunObserved(cfg, maxIterations, prompt, entrypoint, observer)
		var blocked *orchestrator.BlockedError
		if errors.As(runErr, &blocked) {
			now := time.Now().UTC()
			if err := updateMeta(func(meta *session.Metadata) {
				meta.Status = session.StatusBlocked
				meta.NextRun = nil
				meta.BlockReason = blocked.Reason
				meta.ResumeSkill = blocked.Skill
				meta.ResumePrompt = blocked.Prompt
				meta.LastError = ""
				meta.EndedAt = &now
			}); err != nil {
				fmt.Fprintf(os.Stderr, "failed to persist blocked session state: %v\n", err)
			}
			return
		}
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "scheduled run failed: %v\n", runErr)
			if err := updateScheduled(runErr.Error()); err != nil {
				fmt.Fprintf(os.Stderr, "failed to persist scheduled session state: %v\n", err)
			}
			return
		}

		if err := updateScheduled(""); err != nil {
			fmt.Fprintf(os.Stderr, "failed to persist scheduled session state: %v\n", err)
		}
	})
	if err != nil {
		return fmt.Errorf("register cron job: %w", err)
	}

	c.Start()
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSignals()

	<-ctx.Done()

	stopCtx := c.Stop()
	<-stopCtx.Done()

	now := time.Now().UTC()
	if err := updateMeta(func(meta *session.Metadata) {
		meta.Status = session.StatusStopped
		meta.NextRun = nil
		meta.CurrentIteration = 0
		meta.CurrentSkill = ""
		meta.EndedAt = &now
	}); err != nil {
		return fmt.Errorf("persist stopped session state: %w", err)
	}

	return nil
}
