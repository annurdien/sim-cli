## Running Tests

```bash
# Run all tests
go test ./tests/... -v

# Run specific test file
go test ./tests/ -run TestConfig -v

# Run tests with coverage
go test ./tests/... -cover

# Run tests in parallel
go test ./tests/... -parallel 4

# Run performance benchmarks
go test ./tests/... -bench=.
```

