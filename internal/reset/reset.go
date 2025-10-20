package reset

import (
	"fmt"
	"strings"
	"time"

	"code88reset/internal/api"
	"code88reset/internal/models"
	"code88reset/pkg/logger"
)

// Result captures outcome of a reset operation for a single subscription.
type Result struct {
	Subscription        models.Subscription
	ResetResponse       *models.ResetResponse
	Skipped             bool
	SkipReason          string
	Err                 error
	BeforeCredits       float64
	AfterCredits        float64
	BeforeResets        int
	AfterResets         int
	UpdatedSubscription *models.Subscription
	Attempts            int
}

// Filter defines user-selected plan names; empty means all MONTHLY subscriptions.
type Filter struct {
	TargetPlans    []string
	RequireMonthly bool
}

// Options control reset behaviour.
type Options struct {
	ResetType          string
	UseMaxThreshold    bool
	CreditThresholdMax float64
	CreditThresholdMin float64
	SleepBetween       time.Duration
}

// Runner performs reset for all eligible subscriptions under given api client.
type Runner struct {
	client *api.Client
	filter Filter
	opts   Options
}

func NewRunner(client *api.Client, filter Filter, opts Options) *Runner {
	if opts.SleepBetween == 0 {
		opts.SleepBetween = 3 * time.Second
	}
	if opts.ResetType == "" {
		opts.ResetType = "first"
	}
	return &Runner{client: client, filter: filter, opts: opts}
}

// Execute fetches subscriptions, filters them, and resets each eligible one.
func (r *Runner) Execute() ([]Result, error) {
	targets, err := r.Eligible()
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(targets))

	for _, sub := range targets {
		results = append(results, r.processSubscription(sub))
	}

	return results, nil
}

// Eligible returns subscriptions that match filter rules.
func (r *Runner) Eligible() ([]models.Subscription, error) {
	subs, err := r.client.GetSubscriptions()
	if err != nil {
		return nil, err
	}

	return FilterSubscriptions(subs, r.filter), nil
}

// FilterSubscriptions 筛选满足条件的订阅列表。
func FilterSubscriptions(subs []models.Subscription, filter Filter) []models.Subscription {
	if len(subs) == 0 {
		return nil
	}

	targetNames := make(map[string]struct{})
	for _, name := range filter.TargetPlans {
		if trimmed := strings.TrimSpace(strings.ToLower(name)); trimmed != "" {
			targetNames[trimmed] = struct{}{}
		}
	}

	results := make([]models.Subscription, 0, len(subs))
	for _, sub := range subs {
		if filter.RequireMonthly {
			if planType := strings.ToUpper(strings.TrimSpace(sub.SubscriptionPlan.PlanType)); planType != "" && planType != "MONTHLY" {
				continue
			}
		}

		if isPAYGO(sub) {
			continue
		}

		if len(targetNames) > 0 {
			name := strings.ToLower(strings.TrimSpace(sub.SubscriptionName))
			planName := strings.ToLower(strings.TrimSpace(sub.SubscriptionPlan.SubscriptionName))
			if _, ok := targetNames[name]; !ok {
				if _, ok2 := targetNames[planName]; !ok2 {
					continue
				}
			}
		}

		results = append(results, sub)
	}

	return results
}

func (r *Runner) processSubscription(sub models.Subscription) Result {
	result := Result{
		Subscription:  sub,
		BeforeCredits: sub.CurrentCredits,
		BeforeResets:  sub.ResetTimes,
	}

	if skip, reason := r.shouldSkipByThreshold(sub); skip {
		result.Skipped = true
		result.SkipReason = reason
		return result
	}

	if skip, reason := r.shouldSkipByResetTimes(sub); skip {
		result.Skipped = true
		result.SkipReason = reason
		return result
	}

	current := sub
	minRequired := r.minRequiredResetTimes()

	for attempts := 1; attempts <= 2; attempts++ {
		result.Attempts = attempts
		logger.Info("执行重置: subscriptionID=%d (attempt=%d), 当前积分=%.4f, resetTimes=%d",
			current.ID, attempts, current.CurrentCredits, current.ResetTimes)

		resp, err := r.client.ResetCredits(current.ID)
		if err != nil {
			result.Err = err
			return result
		}
		result.ResetResponse = resp

		if r.opts.SleepBetween > 0 {
			time.Sleep(r.opts.SleepBetween)
		}

		updated, fetchErr := r.fetchSubscription(current.ID)
		if fetchErr != nil {
			result.Err = fmt.Errorf("验证订阅 %d 状态失败: %w", current.ID, fetchErr)
			return result
		}

		result.AfterCredits = updated.CurrentCredits
		result.AfterResets = updated.ResetTimes
		result.UpdatedSubscription = updated

		creditsIncreased := result.AfterCredits > result.BeforeCredits
		resetsReduced := result.AfterResets < result.BeforeResets

		if creditsIncreased && resetsReduced {
			return result
		}

		stillEligible := result.AfterResets >= minRequired
		if !stillEligible {
			result.Err = fmt.Errorf("重置后 resetTimes=%d 未减少且不足以继续重置", result.AfterResets)
			return result
		}

		if attempts == 1 {
			logger.Warn("订阅 %d 第一次重置后未确认成功，准备重试", current.ID)
			result.BeforeCredits = updated.CurrentCredits
			result.BeforeResets = updated.ResetTimes
			current = *updated
			continue
		}

		result.Err = fmt.Errorf("多次重置后仍未确认成功 (resetTimes=%d, credits=%.4f)", result.AfterResets, result.AfterCredits)
		return result
	}

	return result
}

func (r *Runner) fetchSubscription(id int) (*models.Subscription, error) {
	subs, err := r.client.GetSubscriptions()
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		if sub.ID == id {
			return &sub, nil
		}
	}
	return nil, fmt.Errorf("未找到订阅 ID=%d", id)
}

func (r *Runner) shouldSkipByThreshold(sub models.Subscription) (bool, string) {
	if sub.SubscriptionPlan.CreditLimit <= 0 {
		return false, ""
	}

	percent := (sub.CurrentCredits / sub.SubscriptionPlan.CreditLimit) * 100

	if r.opts.UseMaxThreshold && r.opts.CreditThresholdMax > 0 {
		if percent > r.opts.CreditThresholdMax {
			return true, fmt.Sprintf("额度充足(%.2f%% > %.1f%%)", percent, r.opts.CreditThresholdMax)
		}
	}

	if !r.opts.UseMaxThreshold && r.opts.CreditThresholdMin > 0 {
		if percent >= r.opts.CreditThresholdMin {
			return true, fmt.Sprintf("额度充足(%.2f%% >= %.1f%%)", percent, r.opts.CreditThresholdMin)
		}
	}

	return false, ""
}

func (r *Runner) shouldSkipByResetTimes(sub models.Subscription) (bool, string) {
	minRequired := r.minRequiredResetTimes()
	if sub.ResetTimes < minRequired {
		return true, fmt.Sprintf("resetTimes不足(需要>=%d)", minRequired)
	}
	return false, ""
}

func (r *Runner) minRequiredResetTimes() int {
	if strings.EqualFold(r.opts.ResetType, "second") {
		return 1
	}
	return 2
}

func isPAYGO(sub models.Subscription) bool {
	if strings.EqualFold(sub.SubscriptionName, "PAYGO") {
		return true
	}
	if strings.EqualFold(sub.SubscriptionPlan.SubscriptionName, "PAYGO") {
		return true
	}
	planType := strings.ToUpper(strings.TrimSpace(sub.SubscriptionPlan.PlanType))
	return planType == "PAYGO" || planType == "PAY_PER_USE"
}
