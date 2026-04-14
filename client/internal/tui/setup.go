package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/config"
)

type setupPhase string

const (
	phaseServerInput setupPhase = "server-input"
	phasePolling     setupPhase = "polling"
	phaseDone        setupPhase = "done"
	phaseError       setupPhase = "error"
)

type createdAuthMsg struct {
	serverURL string
	auth      api.DeviceAuthorization
	err       error
}

type pollResultMsg struct {
	result api.DeviceAuthorizationStatus
	err    error
}

type SetupModel struct {
	paths  config.Paths
	config config.Config

	serverURL     string
	phase         setupPhase
	auth          api.DeviceAuthorization
	authStartedAt time.Time
	err           error
	completed     bool
}

func NewSetupModel(cfg config.Config, paths config.Paths) SetupModel {
	cfg.ApplyDefaults()

	return SetupModel{
		paths:     paths,
		config:    cfg,
		serverURL: cfg.ServerURL,
		phase:     phaseServerInput,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		switch typed.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}

		switch m.phase {
		case phaseServerInput:
			switch typed.String() {
			case "enter":
				serverURL := strings.TrimSpace(m.serverURL)
				if serverURL == "" {
					m.err = fmt.Errorf("server URL is required")
					m.phase = phaseError
					return m, nil
				}

				m.err = nil
				return m, createDeviceAuthCmd(serverURL, m.config.MachineName)
			case "backspace":
				if len(m.serverURL) > 0 {
					m.serverURL = m.serverURL[:len(m.serverURL)-1]
				}
				return m, nil
			default:
				if typed.Type == tea.KeyRunes {
					m.serverURL += typed.String()
				}
				return m, nil
			}
		case phaseDone, phaseError:
			if typed.String() == "enter" || typed.String() == "q" {
				return m, tea.Quit
			}
		}
	case createdAuthMsg:
		if typed.err != nil {
			m.err = typed.err
			m.phase = phaseError
			return m, nil
		}

		m.phase = phasePolling
		m.config.ServerURL = typed.serverURL
		m.auth = typed.auth
		m.authStartedAt = time.Now()
		return m, pollDeviceAuthCmd(typed.serverURL, typed.auth)
	case pollResultMsg:
		if typed.err != nil {
			m.err = typed.err
			m.phase = phaseError
			return m, nil
		}

		switch strings.ToLower(typed.result.State) {
		case "", "pending", "authorization_pending", "created":
			if m.auth.ExpiresInSeconds > 0 && time.Since(m.authStartedAt) >= time.Duration(m.auth.ExpiresInSeconds)*time.Second {
				m.err = fmt.Errorf("device authorization timed out")
				m.phase = phaseError
				return m, nil
			}
			return m, pollDeviceAuthCmd(m.config.ServerURL, m.auth)
		case "approved", "authorized", "complete", "completed":
			if strings.TrimSpace(typed.result.APIKey) == "" {
				m.err = fmt.Errorf("authorization completed without an API key")
				m.phase = phaseError
				return m, nil
			}
			m.config.APIKey = typed.result.APIKey
			m.config.AuthenticatedEmail = typed.result.AuthenticatedEmail
			m.config.AuthenticatedName = typed.result.AuthenticatedName
			if err := config.Save(m.paths.ConfigFile, m.config); err != nil {
				m.err = err
				m.phase = phaseError
				return m, nil
			}
			m.completed = true
			m.phase = phaseDone
			return m, nil
		default:
			m.err = fmt.Errorf("authorization failed: %s", typed.result.State)
			m.phase = phaseError
			return m, nil
		}
	}

	return m, nil
}

func (m SetupModel) View() string {
	switch m.phase {
	case phaseServerInput:
		return "ttime setup\n\n" +
			"Enter the ttime server URL. Press Enter to begin device authorization.\n\n" +
			"Server URL: " + m.serverURL + "\n\n" +
			"Esc or Ctrl+C cancels.\n"
	case phasePolling:
		verificationURL := m.auth.VerificationURL
		if verificationURL == "" {
			verificationURL = m.auth.VerificationURI
		}

		return fmt.Sprintf(
			"ttime setup\n\nOpen this URL in your browser:\n  %s\n\nEnter this code when prompted:\n  %s\n\nPolling for approval every %d seconds. Press Ctrl+C to cancel.\n",
			verificationURL,
			m.auth.UserCode,
			pollIntervalSeconds(m.auth),
		)
	case phaseDone:
		user := m.config.AuthenticatedEmail
		if strings.TrimSpace(user) == "" {
			user = "authorized user"
		}
		return fmt.Sprintf("Setup complete.\n\nServer: %s\nUser:   %s\nInbox:  %s\n\nPress Enter to exit.\n",
			m.config.ServerURL,
			user,
			m.config.InboxDir,
		)
	case phaseError:
		return fmt.Sprintf("Setup failed.\n\n%s\n\nPress Enter to exit.\n", m.err)
	default:
		return "ttime setup\n"
	}
}

func (m SetupModel) Result() (config.Config, error) {
	if m.completed {
		return m.config, nil
	}
	if m.err != nil {
		return config.Config{}, m.err
	}
	return m.config, nil
}

func createDeviceAuthCmd(serverURL, machineName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		client := api.NewClient(serverURL, "")
		auth, err := client.CreateDeviceAuthorization(ctx, machineName)
		return createdAuthMsg{
			serverURL: serverURL,
			auth:      auth,
			err:       err,
		}
	}
}

func pollDeviceAuthCmd(serverURL string, auth api.DeviceAuthorization) tea.Cmd {
	return tea.Tick(time.Duration(pollIntervalSeconds(auth))*time.Second, func(time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		client := api.NewClient(serverURL, "")
		result, err := client.PollDeviceAuthorization(ctx, auth)
		return pollResultMsg{
			result: result,
			err:    err,
		}
	})
}

func pollIntervalSeconds(auth api.DeviceAuthorization) int {
	if auth.IntervalSeconds > 0 {
		return auth.IntervalSeconds
	}
	return 5
}
