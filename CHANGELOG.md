# Changelog

All notable changes to this project are documented in this file.

## Unreleased

### Breaking changes

#### `remotehttpservice.NewForwardingService` signature

`service/target/remotehttpservice.NewForwardingService` now takes a typed `ForwardingServiceConfig` and returns `(ForwardingService, error)`.

**Before:**

```go
svc := remotehttpservice.NewForwardingService(config)
```

where `config` was passed as `interface{}` (often a `ForwardingServiceConfig` value) and the function returned only `ForwardingService`.

**After:**

```go
svc, err := remotehttpservice.NewForwardingService(remotehttpservice.ForwardingServiceConfig{
    Host: cfg.Host,
    // ...
})
if err != nil {
    return err
}
```

Pass `ForwardingServiceConfig` directly (not `interface{}`). Handle the error from construction (for example invalid Sentry DSN configuration). Other fields (`ClientTimeout`, `MaxResponseBodyBytes`, `Logger`, `Sentry`) are optional with defaults documented on the struct.

#### Exponential backoff unrecoverable errors

`util.Backoff.ExecuteWithBackoff` treats an error as non-retryable when `errors.Is(err, util.ErrUnrecoverable)` is true. Legacy code that only includes `util.UnrecoverableErrorMsg` in the error string still stops retrying, but new code should use `fmt.Errorf("...: %w", util.ErrUnrecoverable)`.
