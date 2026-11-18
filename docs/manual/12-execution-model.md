# Execution model

![What happens to one check](run.pdf){width=100%}

## The worker pool

`Run` starts `concurrency` workers (the default when the value is below 1) and queues the checks. A worker takes a check and runs it to its end, so one slow check holds one worker, not the whole run. Results are written by index into a slice sized in advance: there is no result channel that could block, and the order of the results is the order of the bundle.

## Attempts

Each attempt runs under `context.WithTimeout(ctx, timeout)`, so every runner receives a deadline and must return when it ends. A failed attempt is followed by a wait, which honours the context, and a new attempt, up to `retries` attempts in total. There is **no wait after the last attempt**. The longest a check can take is therefore `retries` × `timeout` + (`retries` - 1) × `wait`, a bound the tests assert.

## Cancellation

When the caller's context ends (Ctrl-C in the command, a deadline in a program) the workers stop taking checks, the running attempts end through their context, and every check that did not finish is reported **skipped**. `Run` returns the report it has and the context's error. A skipped check is never a pass: `Report.OK` is false.

## Panics

A panic in a runner (a plug-in with a bug) is recovered, becomes a failed result with an error, and does not touch the other checks (T-10).

## Limits

`Validate` bounds what a bundle may ask for (T-2): 10 attempts, a 5-minute timeout (also the bound of the wait), 256 workers and 1,000 checks. `WithLimits` replaces the bounds for a caller that knows better. `WithAllowedNetworks` and `WithDeniedNetworks` bound where the checks may connect (T-1); the test is made in the dialer's `Control` hook, on the address the connection is about to use, so a DNS answer or a redirect cannot lead out. The icmp check pings the vetted literal address; http checks do not use a proxy when limits are set, because the proxy would hide the destination.

## Memory and time

A run holds the bundle, one result per check and one goroutine per worker. The benchmark in chapter 16 gives reference figures.
