package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/ontai-dev/seam-core/pkg/conditions"
)

// DefaultPollTimeout is the default timeout for ConditionPoller.
const DefaultPollTimeout = 5 * time.Minute

// DefaultPollInterval is the default polling interval for ConditionPoller.
const DefaultPollInterval = 5 * time.Second

// ConditionPoller polls a named Kubernetes CR until a specific status condition
// reaches the expected status, or until the timeout expires.
//
// The condition type must be a known value from seam-core/pkg/conditions.
// Unknown condition types are rejected at construction time, not at poll time,
// so test authors see the error immediately.
//
// Example:
//
//	poller := e2ehelpers.NewConditionPoller(mgmt, rbacProfileGVR, "seam-system",
//	    "platform-rbacprofile", conditions.ConditionTypeReady,
//	    metav1.ConditionTrue, 3*time.Minute, 5*time.Second)
//	Expect(poller.Poll(ctx)).To(Succeed())
type ConditionPoller struct {
	client        *ClusterClient
	gvr           schema.GroupVersionResource
	namespace     string
	name          string
	conditionType string
	wantStatus    metav1.ConditionStatus
	timeout       time.Duration
	interval      time.Duration
}

// NewConditionPoller constructs a ConditionPoller. Returns an error if conditionType
// is not a known value in the shared conditions vocabulary (pkg/conditions).
func NewConditionPoller(
	client *ClusterClient,
	gvr schema.GroupVersionResource,
	namespace, name string,
	conditionType string,
	wantStatus metav1.ConditionStatus,
	timeout, interval time.Duration,
) (*ConditionPoller, error) {
	if err := conditions.ValidateCondition(conditionType, ""); err != nil {
		// ValidateCondition rejects unknown types even with empty reason.
		// We only need to validate the type exists — check the known types list.
		known := conditions.KnownConditionTypes()
		found := false
		for _, ct := range known {
			if ct == conditionType {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("e2e: ConditionPoller: unknown condition type %q: known types are %v",
				conditionType, known)
		}
	}

	if timeout <= 0 {
		timeout = DefaultPollTimeout
	}
	if interval <= 0 {
		interval = DefaultPollInterval
	}

	return &ConditionPoller{
		client:        client,
		gvr:           gvr,
		namespace:     namespace,
		name:          name,
		conditionType: conditionType,
		wantStatus:    wantStatus,
		timeout:       timeout,
		interval:      interval,
	}, nil
}

// Poll blocks until the target condition reaches the desired status or the
// timeout expires. On timeout it returns an error that includes the last
// observed conditions to aid debugging.
func (p *ConditionPoller) Poll(ctx context.Context) error {
	deadline := time.Now().Add(p.timeout)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	var lastConditions string

	for {
		// Check timeout first.
		if time.Now().After(deadline) {
			return fmt.Errorf("e2e: cluster %q: condition %q on %s/%s/%s did not reach %q within %s: last observed conditions: %s",
				p.client.Name, p.conditionType,
				p.gvr.Resource, p.namespace, p.name,
				p.wantStatus, p.timeout, lastConditions)
		}

		// Poll the CR.
		obj, err := p.client.Dynamic.Resource(p.gvr).Namespace(p.namespace).Get(ctx, p.name, metav1.GetOptions{})
		if err != nil {
			// CR may not exist yet — treat as transient and continue polling.
			lastConditions = fmt.Sprintf("<Get error: %v>", err)
		} else {
			conds, observed := extractConditions(obj.Object)
			lastConditions = formatConditions(conds)
			if observed == p.conditionType && string(p.wantStatus) == statusFor(conds, p.conditionType) {
				return nil
			}
			if statusFor(conds, p.conditionType) == string(p.wantStatus) {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("e2e: cluster %q: condition poll cancelled: %w", p.client.Name, ctx.Err())
		case <-ticker.C:
			// next poll iteration
		}
	}
}

// extractConditions reads status.conditions from an unstructured object.
// Returns the raw slice and the first condition type found (for logging).
func extractConditions(obj map[string]interface{}) ([]map[string]interface{}, string) {
	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return nil, ""
	}
	raw, _ := status["conditions"].([]interface{})
	result := make([]map[string]interface{}, 0, len(raw))
	first := ""
	for _, item := range raw {
		if c, ok := item.(map[string]interface{}); ok {
			result = append(result, c)
			if first == "" {
				if t, ok := c["type"].(string); ok {
					first = t
				}
			}
		}
	}
	return result, first
}

// statusFor returns the status string for the named condition type, or "" if absent.
func statusFor(conds []map[string]interface{}, conditionType string) string {
	for _, c := range conds {
		if t, _ := c["type"].(string); t == conditionType {
			s, _ := c["status"].(string)
			return s
		}
	}
	return ""
}

// formatConditions produces a compact summary string for error messages.
func formatConditions(conds []map[string]interface{}) string {
	if len(conds) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		t, _ := c["type"].(string)
		s, _ := c["status"].(string)
		r, _ := c["reason"].(string)
		parts = append(parts, fmt.Sprintf("%s=%s(%s)", t, s, r))
	}
	return strings.Join(parts, ", ")
}
