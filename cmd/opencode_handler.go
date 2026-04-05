package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bnema/openai-accounts-cli/internal/application"
	"github.com/bnema/openai-accounts-cli/internal/domain"
	"github.com/spf13/cobra"
)

type opencodeFailureRequest struct {
	Provider  string `json:"provider"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	AccountID string `json:"account_id"`
}

type opencodeDecisionResponse struct {
	Action    string             `json:"action"`
	RetrySafe bool               `json:"retry_safe"`
	Message   string             `json:"message"`
	Auth      *opencodeAuthEntry `json:"auth,omitempty"`
}

func newOpencodeHandleCmd(app *app) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "handle",
		Short: "Handle OpenCode requests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !jsonOutput {
				return fmt.Errorf("%s: --json is required", cmd.CommandPath())
			}

			var request opencodeFailureRequest
			if err := json.NewDecoder(cmd.InOrStdin()).Decode(&request); err != nil {
				return writeOpencodeDecisionResponse(cmd, opencodeFallbackResponse(fmt.Errorf("decode opencode failure request: %w", err)))
			}

			response, err := handleOpencodeFailure(cmd, app, request)
			if err != nil {
				return writeOpencodeDecisionResponse(cmd, opencodeFallbackResponse(err))
			}

			return writeOpencodeDecisionResponse(cmd, response)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON")
	return cmd
}

func handleOpencodeFailure(cmd *cobra.Command, app *app, request opencodeFailureRequest) (opencodeDecisionResponse, error) {
	if strings.TrimSpace(request.Provider) != "" && request.Provider != opencodeProviderID {
		return opencodeDecisionResponse{
			Action:    string(application.OpencodeRecoveryActionFallback),
			RetrySafe: false,
			Message:   fmt.Sprintf("unsupported provider %q", request.Provider),
		}, nil
	}

	if strings.TrimSpace(request.AccountID) == "" {
		return opencodeDecisionResponse{
			Action:    string(application.OpencodeRecoveryActionFallback),
			RetrySafe: false,
			Message:   "missing account_id; will not retry without current account context",
		}, nil
	}

	decision, err := app.service.DecideOpencodeRecovery(cmd.Context(), domain.AccountID(request.AccountID), request.failure())
	if err != nil {
		return opencodeDecisionResponse{}, fmt.Errorf("decide opencode recovery: %w", err)
	}

	response := opencodeDecisionResponse{
		Action:    string(decision.Action),
		RetrySafe: decision.Retry,
		Message:   opencodeDecisionMessage(decision),
	}

	switch decision.Action {
	case application.OpencodeRecoveryActionRefreshCurrent:
		auth, err := loadOpencodeHandlerAuth(cmd, app, decision.AccountID, true)
		if err != nil {
			return opencodeFallbackResponse(err), nil
		}
		response.Auth = auth
	case application.OpencodeRecoveryActionFailover:
		auth, err := loadOpencodeHandlerAuth(cmd, app, decision.AccountID, false)
		if err != nil {
			return opencodeDecisionResponse{}, err
		}
		response.Auth = auth
	}

	return response, nil
}

func writeOpencodeDecisionResponse(cmd *cobra.Command, response opencodeDecisionResponse) error {
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(response); err != nil {
		return fmt.Errorf("encode opencode decision response: %w", err)
	}

	return nil
}

func (r opencodeFailureRequest) failure() error {
	message := strings.TrimSpace(r.Message)

	switch r.Status {
	case 401, 403:
		if message == "" {
			message = "unauthorized"
		} else {
			message = "unauthorized: " + message
		}
	case 429:
		if message == "" {
			message = "rate limit exceeded"
		}
	}

	if message == "" {
		message = fmt.Sprintf("request failed with status %d", r.Status)
	}

	return errors.New(message)
}

func loadOpencodeHandlerAuth(cmd *cobra.Command, app *app, accountID domain.AccountID, forceRefresh bool) (*opencodeAuthEntry, error) {
	tokens, status, err := loadOAuthTokensForAccount(cmd.Context(), app, accountID)
	if err != nil {
		return nil, err
	}

	updatedTokens, err := ensureFreshTokens(cmd.Context(), app, status.Account, tokens, forceRefresh)
	if err != nil {
		return nil, fmt.Errorf("account %s: refresh oauth tokens: %w", accountID, err)
	}
	tokens = updatedTokens

	entry := opencodeAuthEntry{
		Type:      "oauth",
		Refresh:   tokens.RefreshToken,
		Access:    tokens.AccessToken,
		Expires:   opencodeAuthExpiryMillis(app, tokens),
		AccountID: accountIDFromToken(tokens.IDToken),
	}

	return &entry, nil
}

func opencodeFallbackResponse(err error) opencodeDecisionResponse {
	message := "OpenCode request failed; retry manually after checking account status"
	if err != nil {
		message = err.Error()
	}

	return opencodeDecisionResponse{
		Action:    string(application.OpencodeRecoveryActionFallback),
		RetrySafe: false,
		Message:   message,
	}
}

func opencodeAuthExpiryMillis(app *app, tokens oauthTokens) int64 {
	if tokens.ExpiresAt > 0 {
		return tokens.ExpiresAt * 1000
	}
	if tokens.ExpiresIn > 0 {
		return app.now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UnixMilli()
	}
	return 0
}

func opencodeDecisionMessage(decision application.OpencodeRecoveryDecision) string {
	switch decision.Action {
	case application.OpencodeRecoveryActionRefreshCurrent:
		return fmt.Sprintf("refreshed auth for account %s", decision.AccountID)
	case application.OpencodeRecoveryActionFailover:
		return fmt.Sprintf("switched to account %s", decision.AccountID)
	default:
		switch decision.Class {
		case domain.OpencodeFailureCooldown:
			return "request hit a cooldown; retry later or switch accounts"
		case domain.OpencodeFailureWeeklyLimit:
			return "request hit a weekly limit; switch accounts or wait for reset"
		case domain.OpencodeFailureNoSubscription:
			return "current account has no active subscription"
		case domain.OpencodeFailureAuthInvalid:
			return "current account auth is invalid; reauthenticate the account"
		default:
			return "OpenCode request failed; retry manually after checking account status"
		}
	}
}
