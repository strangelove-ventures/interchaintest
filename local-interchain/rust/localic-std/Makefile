.PHONY: lint
lint:
	@echo "Running linter..."
	@cargo fmt --all
	@cargo clippy -- -D warnings
	@cargo clippy -- -W clippy::pedantic -W clippy::correctness -W clippy::branches_sharing_code