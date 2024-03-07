package checks

import (
	"fmt"
	"time"

	"github.com/dihedron/netcheck/tracked"
)

type TrackedBundle struct {
	id          tracked.Value[string]
	description tracked.Value[string]
	timeout     tracked.Value[Timeout]
	retries     tracked.Value[int]
	wait        tracked.Value[Timeout]
	concurrency tracked.Value[int]
	checks      tracked.Value[[]TrackedCheck]
}

func (g *TrackedBundle) ID() string {
	return g.id.Value()
}

func (g *TrackedBundle) IDAccessed() bool {
	return g.id.Accessed()
}

func (g *TrackedBundle) Description() string {
	return g.description.Value()
}

func (g *TrackedBundle) DescriptionAccessed() bool {
	return g.description.Accessed()
}

func (g *TrackedBundle) Timeout() Timeout {
	return g.timeout.Value()
}

func (g *TrackedBundle) TimeoutAccessed() bool {
	return g.timeout.Accessed()
}

func (g *TrackedBundle) Retries() int {
	return g.retries.Value()
}

func (g *TrackedBundle) RetriesAccessed() bool {
	return g.retries.Accessed()
}

func (g *TrackedBundle) Wait() Timeout {
	return g.wait.Value()
}

func (g *TrackedBundle) WaitAccessed() bool {
	return g.wait.Accessed()
}

func (g *TrackedBundle) Concurrency() int {
	return g.concurrency.Value()
}

func (g *TrackedBundle) ConcurrencyAccessed() bool {
	return g.concurrency.Accessed()
}

func (g *TrackedBundle) Checks() []TrackedCheck {
	return g.checks.Value()
}

func (g *TrackedBundle) ChecksAccessed() bool {
	return g.checks.Accessed()
}

type TrackedCheck struct {
	description tracked.Value[string]
	timeout     tracked.Value[Timeout]
	retries     tracked.Value[int]
	wait        tracked.Value[Timeout]
	address     tracked.Value[string]
	protocol    tracked.Value[Protocol]
	result      tracked.Value[Result]
}

func (g *TrackedCheck) Description() string {
	return g.description.Value()
}

func (g *TrackedCheck) DescriptionAccessed() bool {
	return g.description.Accessed()
}

func (g *TrackedCheck) Timeout() Timeout {
	return g.timeout.Value()
}

func (g *TrackedCheck) TimeoutAccessed() bool {
	return g.timeout.Accessed()
}

func (g *TrackedCheck) Retries() int {
	return g.retries.Value()
}

func (g *TrackedCheck) RetriesAccessed() bool {
	return g.retries.Accessed()
}

func (g *TrackedCheck) Wait() Timeout {
	return g.wait.Value()
}

func (g *TrackedCheck) WaitAccessed() bool {
	return g.wait.Accessed()
}

func (g *TrackedCheck) Address() string {
	return g.address.Value()
}

func (g *TrackedCheck) AddressAccessed() bool {
	return g.address.Accessed()
}

func (g *TrackedCheck) Protocol() Protocol {
	return g.protocol.Value()
}

func (g *TrackedCheck) ProtocolAccessed() bool {
	return g.protocol.Accessed()
}

func (g *TrackedCheck) Result() Result {
	return g.result.Value()
}

func (g *TrackedCheck) ResultAccessed() bool {
	return g.result.Accessed()
}

var MockBundles = []TrackedBundle{
	{
		id:          tracked.New("mock-bundle-1"),
		description: tracked.New("description of mock-bundle-1"),
		timeout:     tracked.New(Timeout(5 * time.Second)),
		retries:     tracked.New(10),
		wait:        tracked.New(Timeout(10 * time.Second)),
		concurrency: tracked.New(20),
		checks: tracked.New([]TrackedCheck{
			{
				description: tracked.New("check-1-1"),
				timeout:     tracked.New(Timeout(1 * time.Second)),
				retries:     tracked.New(10 + 1),
				wait:        tracked.New(Timeout(2 * time.Second)),
				address:     tracked.New("localhost:80"),
				protocol:    tracked.New(TCP),
				result: tracked.New(Result{
					err: nil,
				}),
			},
			{
				description: tracked.New("check-1-2"),
				timeout:     tracked.New(Timeout(1 * time.Second)),
				retries:     tracked.New(10 + 2),
				wait:        tracked.New(Timeout(2 * time.Second)),
				address:     tracked.New("localhost:80"),
				protocol:    tracked.New(UDP),
				result: tracked.New(Result{
					err: fmt.Errorf("error type 1"),
				}),
			},
		}),
	},
	{
		id:          tracked.New("mock-bundle-2"),
		description: tracked.New("description of mock-bundle-2"),
		timeout:     tracked.New(Timeout(5 * time.Second)),
		retries:     tracked.New(30),
		wait:        tracked.New(Timeout(10 * time.Second)),
		concurrency: tracked.New(40),
		checks: tracked.New([]TrackedCheck{
			{
				description: tracked.New("check-2-1"),
				timeout:     tracked.New(Timeout(1 * time.Second)),
				retries:     tracked.New(20 + 1),
				wait:        tracked.New(Timeout(2 * time.Second)),
				address:     tracked.New("localhost:6379"),
				protocol:    tracked.New(TLS),
				result: tracked.New(Result{
					err: nil,
				}),
			},
			{
				description: tracked.New("check-2-2"),
				timeout:     tracked.New(Timeout(1 * time.Second)),
				retries:     tracked.New(20 + 2),
				wait:        tracked.New(Timeout(2 * time.Second)),
				address:     tracked.New("localhost"),
				protocol:    tracked.New(ICMP),
				result: tracked.New(Result{
					err: fmt.Errorf("error type 2"),
				}),
			},
		}),
	},
}
