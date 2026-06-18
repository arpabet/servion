/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servionvrpc

import (
	"fmt"
	"reflect"
	"time"

	"go.arpabet.com/glue"
	"go.arpabet.com/servion"
	"go.arpabet.com/value-rpc/resilience"
	"go.arpabet.com/value-rpc/valuerpc"
)

// ResiliencePolicyClass is the reflect.Type of ResiliencePolicy, used for bean
// lookup.
var ResiliencePolicyClass = reflect.TypeOf((*ResiliencePolicy)(nil)).Elem()

/*
ResiliencePolicy is an optional bean that supplies unary client interceptors —
retry, circuit breaking, timeout, rate limiting, bulkhead, fallback — applied to a
ValueClientFactory client via valueclient.WithInterceptors. When a bean
implementing it is present in the client context the factory installs it
automatically (the same way an ObfsProfile or ConnectAuthorizer bean is picked up).

The interceptors come from go.arpabet.com/value-rpc/resilience; servion only wires
them and implements no governance logic itself, consistent with it being a
bean-assembly framework. The first interceptor in the returned slice is outermost.

Register one with the property-driven ResiliencePolicyFactory, wrap a fixed chain
you build from the resilience package with StaticResiliencePolicy, or implement the
single method on a bean of your own to vary the policy at runtime.

	servionvrpc.StaticResiliencePolicy(
		resilience.CircuitBreaker(),
		resilience.Retry(resilience.WithMaxAttempts(3)),
		resilience.Timeout(2*time.Second),
	)
*/
type ResiliencePolicy interface {
	// Interceptors returns the unary client interceptors to install, outermost
	// first. An empty slice installs nothing.
	Interceptors() []valuerpc.ClientInterceptor
}

// StaticResiliencePolicy returns a ResiliencePolicy bean yielding a fixed list of
// interceptors (built by the caller from the resilience package). Register it in
// the client context to enable governance.
func StaticResiliencePolicy(interceptors ...valuerpc.ClientInterceptor) ResiliencePolicy {
	return &staticResiliencePolicy{interceptors: interceptors}
}

type staticResiliencePolicy struct {
	interceptors []valuerpc.ClientInterceptor
}

func (p *staticResiliencePolicy) Interceptors() []valuerpc.ClientInterceptor {
	return p.interceptors
}

/*
ResiliencePolicyFactory builds a ResiliencePolicy bean from "<beanName>.*"
properties, composing interceptors from go.arpabet.com/value-rpc/resilience. Use
the same beanName as the ValueClientFactory client it governs; the policy is
injected into that client by type.

Recognized properties (all optional; only those present contribute an
interceptor):

	<beanName>.resilience.rate-limit.per-second       >0 enables RateLimit
	<beanName>.resilience.rate-limit.burst            burst (default 1)
	<beanName>.resilience.bulkhead.max-concurrent     >0 enables Bulkhead
	<beanName>.resilience.circuit-breaker.threshold   >0 enables CircuitBreaker
	<beanName>.resilience.circuit-breaker.cooldown-ms cooldown (default 10000)
	<beanName>.resilience.retry.max-attempts          >1 enables Retry
	<beanName>.resilience.retry.backoff-ms            base backoff (default 50)
	<beanName>.resilience.retry.max-backoff-ms        backoff cap (default 1000)
	<beanName>.resilience.timeout-ms                  >0 enables a per-attempt Timeout

Order (outermost first): rate limit -> bulkhead -> circuit breaker -> retry ->
timeout, so the breaker observes whole post-retry operations while the timeout
bounds each attempt. With no matching properties the policy is empty (a no-op).
*/
func ResiliencePolicyFactory(beanName string) glue.FactoryBean {
	return &implResiliencePolicyFactory{beanName: beanName}
}

type implResiliencePolicyFactory struct {
	Properties glue.Properties `inject:""`

	beanName string
}

func (t *implResiliencePolicyFactory) Object() (object interface{}, err error) {

	defer servion.PanicToError(&err)

	key := func(suffix string) string {
		return fmt.Sprintf("%s.resilience.%s", t.beanName, suffix)
	}

	var ics []valuerpc.ClientInterceptor

	if perSec := t.Properties.GetInt(key("rate-limit.per-second"), 0); perSec > 0 {
		burst := t.Properties.GetInt(key("rate-limit.burst"), 1)
		ics = append(ics, resilience.RateLimit(float64(perSec), burst))
	}
	if maxConc := t.Properties.GetInt(key("bulkhead.max-concurrent"), 0); maxConc > 0 {
		ics = append(ics, resilience.Bulkhead(maxConc))
	}
	if threshold := t.Properties.GetInt(key("circuit-breaker.threshold"), 0); threshold > 0 {
		cooldown := t.Properties.GetInt(key("circuit-breaker.cooldown-ms"), 10000)
		ics = append(ics, resilience.CircuitBreaker(
			resilience.WithFailureThreshold(threshold),
			resilience.WithCooldown(time.Duration(cooldown)*time.Millisecond)))
	}
	if maxAttempts := t.Properties.GetInt(key("retry.max-attempts"), 0); maxAttempts > 1 {
		base := t.Properties.GetInt(key("retry.backoff-ms"), 50)
		maxB := t.Properties.GetInt(key("retry.max-backoff-ms"), 1000)
		ics = append(ics, resilience.Retry(
			resilience.WithMaxAttempts(maxAttempts),
			resilience.WithBackoff(
				time.Duration(base)*time.Millisecond,
				time.Duration(maxB)*time.Millisecond)))
	}
	if timeoutMls := t.Properties.GetInt(key("timeout-ms"), 0); timeoutMls > 0 {
		ics = append(ics, resilience.Timeout(time.Duration(timeoutMls)*time.Millisecond))
	}

	return StaticResiliencePolicy(ics...), nil
}

func (t *implResiliencePolicyFactory) ObjectType() reflect.Type { return ResiliencePolicyClass }

// ObjectName ties the policy bean to its client by name; the client injects it by
// type, so the name only needs to be distinct from the client bean itself.
func (t *implResiliencePolicyFactory) ObjectName() string { return t.beanName + ".resilience" }

func (t *implResiliencePolicyFactory) Singleton() bool { return true }
